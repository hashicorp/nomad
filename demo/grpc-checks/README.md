# grpc-checks

An example service that exposes a gRPC healthcheck endpoint

### generate protobuf

Note that main.go also includes this as a go:generate directive
so that running this by hand is not necessary

```bash
$ protoc -I ./health ./health/health.proto --go_out=plugins=grpc:health
```

### build & run example

Generate, compile, and run the example server.

```bash
go generate
go build
go run main.go
```

### publish

#### Testing locally
```bash
$ docker build -t hashicorpnomad/grpc-checks:test .
$ docker run --rm hashicorpnomad/grpc-checks:test
```

#### Upload to Docker Hub
```bash
# replace <version> with the next version number
docker login
$ docker build -t hashicorpnomad/grpc-checks:<version> .
$ docker push hashicorpnomad/grpc-checks:<version>
```
