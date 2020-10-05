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

	"github.com/docker/docker/pkg/ioutils"
	"github.com/hashicorp/go-msgpack/codec"
	"github.com/hashicorp/nomad/nomad/structs"
)

func (s *HTTPServer) EventStream(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
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

	s.parse(resp, req, &args.QueryOptions.Region, &args.QueryOptions)

	// Make the RPC
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

	// Create a goroutine that closes the pipe if the connection closes
	ctx, cancel := context.WithCancel(req.Context())
	defer cancel()
	go func() {
		<-ctx.Done()
		httpPipe.Close()
	}()

	// Create an output that gets flushed on every write
	output := ioutils.NewWriteFlusher(resp)

	// create an error channel to handle errors
	errCh := make(chan HTTPCodedError, 2)

	go func() {
		defer cancel()

		// Send the request
		if err := encoder.Encode(args); err != nil {
			errCh <- CodedError(500, err.Error())
			return
		}

		for {
			select {
			case <-ctx.Done():
				errCh <- nil
				return
			default:
			}

			// Decode the response
			var res structs.EventStreamWrapper
			if err := decoder.Decode(&res); err != nil {
				if err == io.EOF || err == io.ErrClosedPipe {
					return
				}
				errCh <- CodedError(500, err.Error())
				return
			}
			decoder.Reset(httpPipe)

			if err := res.Error; err != nil {
				if err.Code != nil {
					errCh <- CodedError(int(*err.Code), err.Error())
					return
				}
			}

			// Flush json entry to response
			if _, err := io.Copy(output, bytes.NewReader(res.Event.Data)); err != nil {
				errCh <- CodedError(500, err.Error())
				return
			}
		}
	}()

	// invoke handler
	handler(handlerPipe)
	cancel()
	codedErr := <-errCh

	if codedErr != nil &&
		(codedErr == io.EOF ||
			strings.Contains(codedErr.Error(), io.ErrClosedPipe.Error())) {
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

		if topics[structs.Topic(k)] == nil {
			topics[structs.Topic(k)] = []string{v}
		} else {
			topics[structs.Topic(k)] = append(topics[structs.Topic(k)], v)
		}
	}
	return topics, nil
}

func parseTopic(topic string) (string, string, error) {
	parts := strings.Split(topic, ":")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("Invalid key value pair for topic, topic: %s", topic)
	}
	return parts[0], parts[1], nil
}

func allTopics() map[structs.Topic][]string {
	return map[structs.Topic][]string{"*": {"*"}}
}
