// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/pkg/ioutils"
	"github.com/hashicorp/go-msgpack/v2/codec"
	"github.com/hashicorp/nomad/nomad/structs"
	"golang.org/x/sync/errgroup"
)

// deadlineWriter wraps a net.Conn and sets a per-write deadline so writes
// cannot block indefinitely. It implements io.WriteCloser.
type deadlineWriter struct {
	conn    net.Conn
	timeout time.Duration
}

func (w *deadlineWriter) Write(p []byte) (int, error) {
	if w.conn == nil {
		return 0, io.ErrUnexpectedEOF
	}
	_ = w.conn.SetWriteDeadline(time.Now().Add(w.timeout))
	return w.conn.Write(p)
}

func (w *deadlineWriter) Close() error {
	if w.conn == nil {
		return nil
	}
	return w.conn.Close()
}

func (s *HTTPServer) EventStream(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != http.MethodGet {
		return nil, CodedError(http.StatusMethodNotAllowed, ErrInvalidMethod)
	}

	query := req.URL.Query()

	indexStr := query.Get("index")
	if indexStr == "" {
		indexStr = "0"
	}
	index, err := strconv.Atoi(indexStr)
	if err != nil {
		return nil, CodedError(400, fmt.Sprintf("Unable to parse index: %v", err))
	}

	topics, err := parseEventTopics(query)
	if err != nil {
		return nil, CodedError(400, fmt.Sprintf("Invalid topic query: %v", err))
	}

	args := &structs.EventStreamRequest{
		Topics: topics,
		Index:  index,
	}
	resp.Header().Set("Content-Type", "application/json")
	resp.Header().Set("Cache-Control", "no-cache")

	// Set region, namespace and authtoken to args
	s.parse(resp, req, &args.QueryOptions.Region, &args.QueryOptions)

	// Determine the RPC handler to use to find a server
	var handler structs.StreamingRpcHandler
	var handlerErr error
	if server := s.agent.Server(); server != nil {
		handler, handlerErr = server.StreamingRpcHandler("Event.Stream")
	} else if client := s.agent.Client(); client != nil {
		handler, handlerErr = client.RemoteStreamingRpcHandler("Event.Stream")
	} else {
		handlerErr = fmt.Errorf("misconfigured connection")
	}

	if handlerErr != nil {
		return nil, CodedError(500, handlerErr.Error())
	}

	httpPipe, handlerPipe := net.Pipe()
	decoder := codec.NewDecoder(httpPipe, structs.MsgpackHandle)
	encoder := codec.NewEncoder(httpPipe, structs.MsgpackHandle)

	// writeTimeout is set to 10 seconds more than the heartbeat of
	// NewJsonStream
	writeTimeout := 40 * time.Second

	ctx, cancel := context.WithCancel(req.Context())
	defer cancel()

	var output io.WriteCloser
	if hj, ok := resp.(http.Hijacker); ok {
		conn, bufrw, err := hj.Hijack()
		if err == nil {
			// Build response headers for the hijacked connection.
			// merge any user-configured headers from the agent config
			// (http_api_response_headers).
			headers := resp.Header().Clone()
			if cfg := s.agent.GetConfig(); cfg != nil {
				for k, v := range cfg.HTTPAPIResponseHeaders {
					headers.Set(k, v)
				}
			}

			// we inherit gzip from the agent, but we don't really respond with
			// gzip
			headers.Del("Content-Encoding")

			// Write the HTTP response to the hijacked connection and ensure we
			// set the same protocol version as the incoming request.
			res := &http.Response{
				Status:        "200 OK",
				StatusCode:    http.StatusOK,
				Proto:         req.Proto,
				ProtoMajor:    req.ProtoMajor,
				ProtoMinor:    req.ProtoMinor,
				Header:        headers,
				ContentLength: -1,
			}

			_ = conn.SetWriteDeadline(time.Now().Add(writeTimeout))
			_ = res.Write(conn)

			// If the hijack returned a buffered writer/reader, flush any
			// buffered data so middleware or the net/http stack doesn't
			// leave pending bytes unflushed on the connection. Flush after
			// writing headers to preserve header ordering.
			if bufrw != nil {
				_ = bufrw.Flush()
			}

			output = ioutils.NewWriteFlusher(&deadlineWriter{conn: conn, timeout: writeTimeout})
		}
	}
	if output == nil {
		// Fallback: the existing flusher (no-op Close).
		output = ioutils.NewWriteFlusher(resp)
	}

	// Create a goroutine that closes the pipe if the connection closes
	go func() {
		<-ctx.Done()
		httpPipe.Close()
		output.Close()
	}()

	// send request and decode events
	errs, errCtx := errgroup.WithContext(ctx)
	errs.Go(func() error {
		defer cancel()

		// Send the request
		if err := encoder.Encode(args); err != nil {
			return CodedError(500, err.Error())
		}

		for {
			select {
			case <-errCtx.Done():
				return nil
			default:
			}

			// Decode the response
			var res structs.EventStreamWrapper
			if err := decoder.Decode(&res); err != nil {
				return CodedError(500, err.Error())
			}
			decoder.Reset(httpPipe)

			if err := res.Error; err != nil {
				if err.Code != nil {
					return CodedError(int(*err.Code), err.Error())
				}
			}

			// Flush json entry to response
			if _, err := io.Copy(output, bytes.NewReader(res.Event.Data)); err != nil {
				return CodedError(500, err.Error())
			}
			// Each entry is its own new line according to https://github.com/ndjson/ndjson-spec
			// append new line to each entry
			fmt.Fprint(output, "\n")
		}
	})

	// invoke handler
	handler(handlerPipe)
	cancel()

	codedErr := errs.Wait()
	if codedErr != nil && strings.Contains(codedErr.Error(), io.ErrClosedPipe.Error()) {
		codedErr = nil
	}

	return nil, codedErr
}

func parseEventTopics(query url.Values) (map[structs.Topic][]string, error) {
	raw, ok := query["topic"]
	if !ok {
		return allTopics(), nil
	}
	topics := make(map[structs.Topic][]string)

	for _, topic := range raw {
		k, v, err := parseTopic(topic)
		if err != nil {
			return nil, fmt.Errorf("error parsing topics: %w", err)
		}

		topics[structs.Topic(k)] = append(topics[structs.Topic(k)], v)
	}
	return topics, nil
}

func parseTopic(topic string) (string, string, error) {
	parts := strings.Split(topic, ":")
	// infer wildcard if only given a topic
	if len(parts) == 1 {
		return topic, "*", nil
	} else if len(parts) != 2 {
		return "", "", fmt.Errorf("Invalid key value pair for topic, topic: %s", topic)
	}
	return parts[0], parts[1], nil
}

func allTopics() map[structs.Topic][]string {
	return map[structs.Topic][]string{"*": {"*"}}
}
