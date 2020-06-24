FROM golang:alpine as builder
WORKDIR /build
ADD ./Dockerfile /build

RUN apk add git && \
    git clone -b 0.0.3 https://github.com/hashicorp/demo-consul-101.git

WORKDIR /build/demo-consul-101/services/counting-service
ADD ./go.mod .

RUN go version && \
    go env && \
    go generate && \
    GO111MODULE=on CGO_ENABLED=0 GOOS=linux go build

FROM alpine:latest
MAINTAINER nomadproject.io

WORKDIR /opt
COPY --from=builder /build/demo-consul-101/services/counting-service/counter-api /opt

EXPOSE 9001
ENV PORT 9001

ENTRYPOINT ["/opt/counter-api"]
