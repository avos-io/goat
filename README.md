# goat

GOAT is gRPC Over Any Transport.

The basic idea is to serialise a gRPC request into a wrapper protobuf that also includes any metadata needed to describe the request. We can then implement gRPC over any transport capable of providing an interface for reading and writing those wrapped RPcs. It's just like JSON-RPC, but using protobufs and for gRPC.

# Language implementations

## Golang

Golang is our canonical language for GOAT and is implemented in this repo.

## Typescript / ECMAScript

See [here](https://github.com/avos-io/goat-es).

## Kotlin

TODO

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
