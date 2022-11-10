# goat

GOAT is gRPC Over Any Transport.

The basic idea is to serialise a gRPC request into a wrapper protobuf that also includes any metadata needed to describe the request. We can then implement gRPC over any transport capable of providing an interface for reading and writing those wrapped RPcs. It's just like JSON-RPC, but using protobufs and for gRPC.
