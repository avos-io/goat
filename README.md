# 🐐 GOAT 🐐

GOAT is gRPC Over Any Transport. The standard GRPC implementations are quite closely tied to HTTP/2, but there are cases where such a transport is not optimal or not possible. GOAT is an attempt at a common abstraction layer over the networking portion, allowing GRPC to work over any reliable transport: WebSockets, TCP, a `pipe(2)`, etc.

The basic idea is to serialise a gRPC request into a wrapper protobuf that also includes any metadata needed to describe the request. The wrapper contains the same sort of information provided in HTTP/2 headers, trailers, method name, and stream ID. The libraries in this and linked repositories then use this to provide client and server interfaces.

# Language implementations

## Golang

Golang is our canonical language for GOAT and is implemented in this repo. See below for more information on using it.

## Typescript / ECMAScript

See the [goat-es](https://github.com/avos-io/goat-es) repository.

## Kotlin

Work in progress. Come back here later.

# GOAT for Golang

GOAT works with the standard GRPC libraries for Go, implementing the [ClientConnInterface](https://pkg.go.dev/google.golang.org/grpc#ClientConnInterface) and a `Server` (that handles `*ServiceDesc`) interfaces for clients and servers respectively.

The transport interface is very minimal:

```go
type RpcReadWriter interface {
	Read(context.Context) (*goatorepo.Rpc, error)
	Write(context.Context, *goatorepo.Rpc) error
}
```

See [Client](#client) below on how to use a GOAT client. See [Server](#server) below on how to use a GOAT server.

----

## Concepts

### Transports

A _transport implementation_ is one implementing this interface; two example implementations exist in this repo:
* A simple websocket-based tranport in `NewGoatOverWebsocket()`. This allows creating a new GOAT transport given a websocket connection from the [nhooyr.io/websocket](https://nhooyr.io/websocket) Golang module.
* The unit testing code has several transport implementations, e.g. `NewGoatOverPipe()` which works over any `net.Conn` including those returned by [net.Pipe()](https://pkg.go.dev/net#Pipe).
* An example transport over HTTP 1/1.1 in `NewGoatOverHttp()`. You probably don't want to use this, but it serves as another example of implementing a transport.

### Client and server names

GOAT allows clients to include an identifying name, and to specify the name of a server destination:
* Client source names are not used for anything inside the GOAT implementation, and may be used by users of this library for cases like complex proxying.
* Destination and server names must match, else GOAT will reject an RPC. If names are not desirable, then the client and server should specify an empty string for the destination and server names respectively.

Names are designed for improved debugging, logging, and ability to create more complex routing or proxying setups.

### Client

The client side only requires a transport implementation instance, and then provides regular GRPC usage like normal.

### Server

The server side allows binding GRPC services like normal. It is then invoked to serve each client individually.

### Proxy

It is possible to build proxies much like HTTP reverse proxies can be used with GRPC over HTTP.

----

## Client

Consider the example regular Go GRPC example code as follows - taken from [the GRPC docs](https://grpc.io/docs/languages/go/basics/).

```go
var opts []grpc.DialOption
// ...
conn, err := grpc.Dial(*serverAddr, opts...)
if err != nil {
    // ...
}
defer conn.Close()

client := pb.NewRouteGuideClient(conn)

feature, err := client.GetFeature(context.Background(), &pb.Point{409146138, -746188906})
if err != nil {
    // ...
}
```

To change this to use a GOAT based transport, we simply change it like so:

```go
websock, _, err := websocket.Dial(ctx, "ws://localhost:8080", nil)
if err != nil {
	// ...
}
defer websock.CloseNow()

var opts []goat.DialOption
// ...
goatConn := goat.NewGoatOverWebsocket(websock)
conn := goat.NewClientConn(goatConn, "", "srv", opts...)

client := pb.NewRouteGuideClient(conn)

feature, err := client.GetFeature(context.Background(), &pb.Point{409146138, -746188906})
if err != nil {
    // ...
}
```

GOAT is used in two ways then: creating a new `RpcReadWriter` (in this case via `goat.NewGoatOverWebsocket()`) and then using `goat.NewClientConn()`. This returns something that implements `ClientConnInterface`, satisfying the needs of the generated code that creates a new GRPC service client.

## Server

Consider the example regular Go GRPC example code as follows - taken from [the GRPC docs](https://grpc.io/docs/languages/go/basics/).

```go
lis, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", port))
if err != nil {
    log.Fatalf("failed to listen: %v", err)
}
var opts []grpc.ServerOption
// ...
grpcServer := grpc.NewServer(opts...)
pb.RegisterRouteGuideServer(grpcServer, newServer())
grpcServer.Serve(lis)
```

Rather than `grpc.NewServer()` we use `goat.NewServer()`. 

```go
// ...
grpcServer := goat.NewServer("srv")
pb.RegisterRouteGuideServer(grpcServer, newServer())
// ...
// For each new client connection coming in, we call grpcServer.Serve():
grpcServer.Serve(context.TODO(), clientRpcReadWriter)
```

### Proxy

GOAT is designed to be able to build proxies in the same manner as HTTP reverse proxies. This Golang library supports such use-cases via the `goat.NewProxy()` function.

This is an advanced feature and not yet documented. It is recommended to read the code and tests in this area.

# Protocol

GRPC is canonically [transported over HTTP/2](https://github.com/grpc/grpc/blob/master/doc/PROTOCOL-HTTP2.md), making use of HTTP/2's ability to provide framing and stream management. 

Three concepts from HTTP/2 are core to understanding GRPC's use there (we're glossing over detail like `CONTINUATION` frames and more; the following suffices):
1. `HEADERS` frames
2. `DATA` frames
3. [Stream Identifiers](https://datatracker.ietf.org/doc/html/rfc7540#section-5.1.1) and `END_STREAM` flag

`HEADERS` frames are used to begin a request, to begin answering a request, and to finalise a response by including "trailers" and a status code. `DATA` frames contain the actual payload RPC data.

These two types of frames are logically associated by a _Stream Identifier_, a 31-bit integer used to demarcate different streams. And to signal the end of request or response, the `END_STREAM` flag is specified.

Here we provide a generic framing and stream management protocol, allowing GRPC to be serialised over any reliable transport. We take inspiration from the HTTP/2 protocol and provide minimal protocol with complementary semantics.

## Data format

Data is formatted in Protobuf messages: this means RPC requests can effectively have two layers of protobuf encoding: the information used here to identify a request in the GOAT protocol, and the usual RPC data itself (that found in the `DATA` frame normally). The precise format is referenced in the source here, but it approximately looks like this:

```protobuf
message Rpc {
    // Analogous to a "Stream Identifier" in HTTP/2: allocated by the initator of an RPC, a
    // unique identifier used to group requests and responses specific to this request.
    uint64 id = 1;
    // Information identifying the request to be made; hence always set in the initial
    // request. At a minimum in such a request, the RPC method to be invoked is set. This
    // information is transferred in the URL path in HTTP/2.
    RequestHeader header = 2;
    // When a request finishes, it is explicitly marked with a status code, and optionally
    // with trailers. In HTTP/2 this is communicated with a HEADERS frame following the
    // response DATA frame, with certain canonical headers set like `grpc-status`.
    ResponseStatus status = 3;
    // The actual RPC request or response data: just some opaque bytes, this is usually
    // protobuf-serialised bytes.
    Body body = 4;
    // Like status, this is sent as part of a response, and allows arbitrary key/values
    // to be communicated. This maps onto "Trailers" in the HTTP/2 encoding, but without
    // status-code, which is encoded explicitly above.
    Trailer trailer = 5;
}
```

Hence a GOAT communication includes a series of `Rpc` messages in both directions, multiple RPCs multiplexed by the `id` parameter which is always set.

## Unary

A unary request will contain:
* `id`: always specified
* `header`: itself with `method` always set
* `body`: itself always specified, but may be empty if the request contains no data

And a response:
* `id`: always specified
* `header`: at a minimal echoes back the `method` called
* `status`: usually only set on error
* `body`: always specified, but may be empty, if the response contains no data
* `trailer`: always specified, but may be empty, but its presence signals the request has finished

## Streaming

Streaming RPCs need to additionally be able to signal streaming-start and streaming-stop concepts:
* Starting is explicit by virtue of a new `id` being used for a method, and the initial request having no body and no trailers
* Stopping is always indicated -- in both directions -- by presence of the `trailer`

Because streaming can be in any of three configurations (server, client, bidir), there are now cases where a request or response does not need to carry a `body`. Included below are examples of what requests and responses might flow in each case. Note that exact ordering of messages can differ from the examples below.

### Server stream

In this case the client sends just a single message, and initiates a streaming download. Therefore the requests/responses look something like:
1. Client ➞ Server: `id`, `header`
2. Client ➞ Server: `id`, `header`, `body`?, `trailer`
3. Client 🠔 Server: `id`, `header`, `body` (repeated)
4. Client 🠔 Server: `id`, `header`, `body`?, `status`?, `trailer`

It is valid for a server to respond without ever specifying a body, and only ever specifying `trailer` (and probably `status`).

### Client stream

In this case the client uploads data to the server, which itself does not respond with any data:
1. Client ➞ Server: `id`, `header`
2. Client ➞ Server: `id`, `header`, `body` (repeated)
3. Client ➞ Server: `id`, `header`, `status`?, `trailer`
4. Client 🠔 Server: `id`,  `header`, `body`, `status`?, `trailer`

It is valid for the client to specify a `status` to the server when it stops streaming, allowing the server to disambiguate expected and unexpected cases of the streaming stopping.

Once the client finishes streaming by specifying a `trailer`, the server can respond with `body` and `trailer`.

### Bidirectional stream

Bidirectional streams allow data-flow in both directions between clients and servers:
1. Client ➞ Server: `id`, `header`
2. Client ➞ Server: `id`, `header`, `body` (repeated)
3. Client 🠔 Server: `id`, `header`, `body` (repeated)
4. Client 🠔 Server: `id`, `header`, `status`?, `trailer`
5. Client ➞ Server: `id`, `header`, `status`?, `trailer`

The order of closing is up to the application.
