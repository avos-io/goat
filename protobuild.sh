# macOS protobuf can't find built-in types
if [[ "$OSTYPE" == "darwin"* ]]; then
    grpc_opts="$grpc_opts --proto_path /usr/local/include --proto_path ."
fi

# A note on "--go-grpc_opt=requireUnimplementedServers=false": see the big
# discussion here: https://github.com/grpc/grpc-go/issues/3669
#
# And documentation here:
# https://github.com/grpc/grpc-go/blob/master/cmd/protoc-gen-go-grpc/README.md
#
# At least in the case of using mockery (https://github.com/vektra/mockery),
# the default mode won't work and we need to use this option.
if [ -n "${DISABLE_REQUIRE_UNIMPLEMENTED_SERVERS}" ]; then
    grpc_opts="$grpc_opts --go-grpc_opt=require_unimplemented_servers=false"
    echo "==> disabling require_unimplemented_servers"
fi

mkdir -p gen
protoc --go_out=gen --go_opt=paths=source_relative --go-grpc_out=gen --go-grpc_opt=paths=source_relative ./goat.proto $grpc_opts

mkdir -p gen/testproto
protoc --go_out=gen/testproto --go_opt=paths=source_relative --go-grpc_out=gen/testproto --go-grpc_opt=paths=source_relative ./test.proto $grpc_opts
