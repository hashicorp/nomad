generate:
	protoc -I proto/ proto/raw-exec.proto --go_out=plugins=grpc:proto/

build-plugin:
	go build -o raw-exec-go-grpc ./plugin-go-grpc

build-cli:
	go build -o raw_exec

run:
	./raw_exec
