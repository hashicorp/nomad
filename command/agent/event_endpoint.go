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
	"github.com/hashicorp/nomad/lib/lang"
	"github.com/hashicorp/nomad/nomad/structs"
	"golang.org/x/sync/errgroup"
)

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

	// Create an output that gets flushed on every write
	output := ioutils.NewWriteFlusher(resp)

	// Create a heartbeat that is just a bit longer than NewJsonStream and close the
	// connection when it ticks
	writeTimeout := 40 * time.Second
	heartbeat := time.NewTicker(writeTimeout)
	defer heartbeat.Stop()

	// Create a goroutine that closes the pipe if the connection closes
	ctx, cancel := context.WithCancel(req.Context())
	defer cancel()

	go func() {
		select {
		case <-ctx.Done():
		case <-heartbeat.C:
			s.logger.Debug("event endpoint: heartbeat passed, closing pipes and canceling the context")
			cancel()
		}

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
			heartbeat.Reset(writeTimeout)

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

			// Flush json entry to response, and make sure we stop reading if the ctx
			// cancels (otherwise io.Copy blocks forever in case there's backpressure on
			// the endpoint)
			if _, err := io.Copy(output, lang.NewCtxReader(ctx, bytes.NewReader(res.Event.Data))); err != nil {
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
