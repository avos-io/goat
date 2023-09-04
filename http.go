package goat

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/proto"
)

// OnConnect can be called by the GoatOverHttp to indicate that a new client has
// connected.
type OnConnect func(id string, rw RpcReadWriter)

// SourceToAddress maps an Rpc source into an addressable entity.
type SourceToAddress func(src string) (string, error)

// GoatOverHttp manages GOAT connections over HTTP 1.0, yielding a new
// HTTPRpcReadWriter whenever it receives an Rpc from a new source (via
// ServeHTTP) or when prompted to establish a NewConnection.
type GoatOverHttp struct {
	ctx    context.Context
	cancel context.CancelFunc

	onConnect       OnConnect
	sourceToAddress SourceToAddress

	connectionCleanupInterval time.Duration
	connectionTimeout         time.Duration
	clock                     clock.Clock

	conns struct {
		sync.Mutex
		value map[string]*httpReadWriter
	}
}

func NewGoatOverHttp(
	onConnect OnConnect,
	sourceToAddress SourceToAddress,
	opts ...GoatOverHttpOption,
) *GoatOverHttp {
	ctx, cancel := context.WithCancel(context.Background())

	goh := &GoatOverHttp{
		ctx:    ctx,
		cancel: cancel,

		onConnect:       onConnect,
		sourceToAddress: sourceToAddress,

		connectionCleanupInterval: 1 * time.Minute,
		connectionTimeout:         4 * time.Minute,
		clock:                     clock.New(),
	}
	goh.conns.value = make(map[string]*httpReadWriter)

	for _, opt := range opts {
		opt.apply(goh)
	}

	go goh.connectionCleaner()

	return goh
}

type GoatOverHttpOption interface {
	apply(*GoatOverHttp)
}

type goatOverHttpOptFunc func(*GoatOverHttp)

func (fn goatOverHttpOptFunc) apply(goh *GoatOverHttp) {
	fn(goh)
}

// WithClock sets the clock of the GoatOverHttp.
func WithClock(cl clock.Clock) GoatOverHttpOption {
	return goatOverHttpOptFunc(func(goh *GoatOverHttp) {
		goh.clock = cl
	})
}

// WithConnectionCleanupInterval sets the interval that the GoatOverHttp will
// wait between firings of the connection cleanup routine, which will close any
// stale connections.
func WithConnectionCleanupInterval(i time.Duration) GoatOverHttpOption {
	return goatOverHttpOptFunc(func(goh *GoatOverHttp) {
		goh.connectionCleanupInterval = i
	})
}

// WithConnectionTimeout sets the duration after which the GoatOverHttp will
// consider a given connection as stale and in need of being closed.
func WithConnectionTimeout(t time.Duration) GoatOverHttpOption {
	return goatOverHttpOptFunc(func(goh *GoatOverHttp) {
		goh.connectionTimeout = t
	})
}

func (goh *GoatOverHttp) Cancel() {
	log.Info().Msg("GoatOverHttp: Cancel")
	goh.cancel()
}

func (goh *GoatOverHttp) NewConnection(destination string) RpcReadWriter {
	conn, _ := goh.retrieve(destination)
	return conn
}

func (goh *GoatOverHttp) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Body == nil {
		log.Error().Msg("GoatOverHttp: missing body")
		http.Error(w, "missing body", http.StatusBadRequest)
		return
	}

	data, err := io.ReadAll(r.Body)

	r.Body.Close()

	if err != nil {
		log.Error().Msg("GoatOverHttp: failed to read body")
		http.Error(w, "body read", http.StatusBadRequest)
		return
	}

	var rpc Rpc
	err = proto.Unmarshal(data, &rpc)
	if err != nil {
		log.Error().Msgf("GoatOverHttp: cannot decode rpc")
		http.Error(w, "proto decode", http.StatusBadRequest)
		return
	}

	if rpc.Header == nil || rpc.Header.Source == "" {
		log.Error().Msgf("GoatOverHttp: missing header/source: %v", &rpc)
		http.Error(w, "missing header/source", http.StatusBadRequest)
		return
	}

	source, err := goh.sourceToAddress(rpc.Header.Source)
	if err != nil {
		log.Error().Msgf("GoatOverHttp: failed to map source: %s", rpc.Header.Source)
		http.Error(w, "failed to map source", http.StatusBadRequest)
		return
	}

	conn, new := goh.retrieve(source)

	if new {
		go goh.onConnect(source, conn)
	}

	conn.readCh <- &rpc
}

// connectionCleaner ticks every |connectionCleanupInterval|, closing any
// connections whose last activity is older than |connectionTimeout|.
func (goh *GoatOverHttp) connectionCleaner() {
	ticker := goh.clock.Ticker(goh.connectionCleanupInterval)
	for {
		select {
		case <-goh.ctx.Done():
			ticker.Stop()
			return
		case <-ticker.C:
			now := goh.clock.Now().Unix()

			goh.conns.Lock()
			for _, conn := range goh.conns.value {
				if now-conn.lastActivity.Load() >= int64(goh.connectionTimeout.Seconds()) {
					log.Info().Msgf("GoatOverHttp: timing out conn to %s", conn.writeAddr)
					goh.unregisterLocked(conn.writeAddr)
				}
			}
			goh.conns.Unlock()
		}
	}
}

// retrieve returns a connection for the id and whether it was newly created.
func (goh *GoatOverHttp) retrieve(id string) (*httpReadWriter, bool) {
	goh.conns.Lock()
	defer goh.conns.Unlock()

	conn, ok := goh.conns.value[id]

	if !ok {
		conn = &httpReadWriter{
			writeAddr: id,
			readCh:    make(chan *Rpc),
			cancel:    func() { goh.unregister(id) },
			clock:     goh.clock,
		}

		goh.conns.value[id] = conn
	}

	return conn, !ok
}

func (goh *GoatOverHttp) unregister(id string) {
	goh.conns.Lock()
	defer goh.conns.Unlock()

	goh.unregisterLocked(id)
}

func (goh *GoatOverHttp) unregisterLocked(id string) {
	if conn, ok := goh.conns.value[id]; ok {
		close(conn.readCh)
	}

	delete(goh.conns.value, id)
}

type httpReadWriter struct {
	writeAddr string
	readCh    chan *Rpc
	cancel    func()

	clock        clock.Clock
	lastActivity atomic.Int64
}

func (hrw *httpReadWriter) Read(ctx context.Context) (*Rpc, error) {
	rpc, ok := <-hrw.readCh
	if !ok {
		log.Error().Msgf("HttpRpcReadWriter: read err: closed")
		return nil, errors.New("readCh closed")
	}
	hrw.bumpActivity()
	return rpc, nil
}

func (hrw *httpReadWriter) Write(ctx context.Context, rpc *Rpc) error {
	hrw.bumpActivity()

	data, err := proto.Marshal(rpc)
	if err != nil {
		return err
	}

	scheme := "http"
	if strings.HasSuffix(hrw.writeAddr, ":443") {
		scheme = "https"
	}

	r, err := http.NewRequest("POST", scheme+"://"+hrw.writeAddr, bytes.NewBuffer(data))
	if err != nil {
		hrw.cancel()
		return err
	}

	r.Header.Add("Content-Type", "application/octet-stream")
	r.Header.Add("X-Avos-Goat-Version", "1.0")

	client := &http.Client{}
	resp, err := client.Do(r)

	if err != nil {
		log.Error().Err(err).Msgf("HttpRpcReadWriter: failed to write")
		// TODO: retry
		hrw.cancel()
		return err
	}

	resp.Body.Close()

	return nil
}

func (hrw *httpReadWriter) bumpActivity() {
	hrw.lastActivity.Store(hrw.clock.Now().Unix())
}
