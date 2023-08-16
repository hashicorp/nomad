FROM docker.io/library/golang:1.21rc4-alpine as builder

#RUN gocat unix-to-tcp --src api.sock --dst 0.0.0.0:4000
RUN go install github.com/sumup-oss/gocat@v0.2.0

COPY api /code/api
WORKDIR /code/api
RUN go get

WORKDIR /code
COPY go.* ./
RUN go mod download
COPY *.go ./
RUN go build -o /go/bin/sock-locker .

FROM docker.io/library/golang:1.21rc4-alpine

WORKDIR /go/bin
COPY --from=builder /go/bin/gocat .
COPY --from=builder /go/bin/sock-locker .

CMD ["sock-locker"]
