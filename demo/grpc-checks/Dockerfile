FROM golang:alpine as builder
WORKDIR /build
ADD . /build
RUN apk add protoc && \
    go get -u github.com/golang/protobuf/protoc-gen-go
RUN go version && \
    go env && \
    go generate && \
    CGO_ENABLED=0 GOOS=linux go build

FROM alpine:latest
MAINTAINER nomadproject.io

WORKDIR /opt
COPY --from=builder /build/grpc-checks /opt

ENTRYPOINT ["/opt/grpc-checks"]

