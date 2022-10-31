# grpc-websockets

Playground trying to implement GRPCs over a single Websocket.

The basic idea is to serialise a GRPC request into a wrapper protobuf that also includes any metadata needed to describe the request. It's just like JSON-RPC, but using protobufs and for GRPC.

## Golang

### Client

Unary implementation exists. This just needs to implement `grpc.ClientConnInterface`. This is the interface passed into the `New*ServiceClient` generated GRPC code -- so no change is needed by getting a pointer to one of these rather than calling `grpc.DialContext()`.

Most of the work is in `internal/multiplexer` for this.

TODO: streaming. Metadata (Headers/Trailers).

### Server

Crude unary implementation exists. The server side is somewhat less obvious as it's more involved than just implementing one interface. Here we implement `grpc.ServiceRegistrar` allowing our service to be a target of the generated bindings `Register*ServiceServer`. The implementation of this just copies the service registration information and ultimately calls it fairly directly.

The annoying piece of that puzzle is we need to reproduce a bunch of basic code that lives in the grpc repo - there doesn't appear to be anything better to do than copy and paste it. It's not heaps of code, but it's annoying there is no way to compose/use their code easily.

TODO: streaming. Error handling. Metadata (Headers/Trailers). Interceptors.

### Testing

No end-to-end test beyond whatever exists in `main.go`. This should not be difficult to compose some unit tests for. We should test the various cases of producing errors, as well as the various types of RPCs (unary, the various streaming types), and metadata.

## Web / Typescript

TODO
