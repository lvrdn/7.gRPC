get:
	go get google.golang.org/grpc
	go get google.golang.org/protobuf/cmd/protoc-gen-go
	go get google.golang.org/grpc/cmd/protoc-gen-go-grpc

gen:
	protoc --go_out=pkg/generated/ --go-grpc_out=require_unimplemented_servers=false:pkg/generated/ api/service.proto

.PHONY: test
test:
	go test ./test -v -race