# macOS protobuf can't find built-in types
if [[ "$OSTYPE" == "darwin"* ]]; then
    grpc_opts="$grpc_opts --proto_path /usr/local/include --proto_path ."
fi

mkdir -p gen
protoc --go_out=gen --go_opt=paths=source_relative --go-grpc_out=gen --go-grpc_opt=paths=source_relative ./rpcheader.proto $grpc_opts
protoc --go_out=gen --go_opt=paths=source_relative --go-grpc_out=gen --go-grpc_opt=paths=source_relative ./test.proto $grpc_opts
