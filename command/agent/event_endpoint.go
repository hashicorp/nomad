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
	"golang.org/x/sync/errgroup"
)

func (s *HTTPServer) EventSinksRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != http.MethodGet {
		return nil, CodedError(http.StatusMethodNotAllowed, ErrInvalidMethod)
	}

	args := structs.EventSinkListRequest{}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var out structs.EventSinkListResponse
	if err := s.agent.RPC("Event.ListSinks", &args, &out); err != nil {
		return nil, err
	}

	if out.Sinks == nil {
		out.Sinks = make([]*structs.EventSink, 0)
	}
	setMeta(resp, &out.QueryMeta)
	return out.Sinks, nil
}

func (s *HTTPServer) EventSinkSpecificRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	name := strings.TrimPrefix(req.URL.Path, "/v1/event/sink/")
	if len(name) == 0 {
		return nil, CodedError(http.StatusBadRequest, "Missing Policy Name")
	}
	switch req.Method {
	case http.MethodGet:
		return s.eventSinkGet(resp, req, name)
	case http.MethodPost, http.MethodPut:
		return s.eventSinkUpdate(resp, req, name)
	case http.MethodDelete:
		return s.eventSinkDelete(resp, req, name)
	default:
		return nil, CodedError(405, ErrInvalidMethod)
	}
}

func (s *HTTPServer) eventSinkGet(resp http.ResponseWriter, req *http.Request, sink string) (interface{}, error) {
	args := structs.EventSinkSpecificRequest{
		ID: sink,
	}
	if s.parse(resp, req, &args.Region, &args.QueryOptions) {
		return nil, nil
	}

	var out structs.EventSinkResponse
	if err := s.agent.RPC("Event.GetSink", &args, &out); err != nil {
		return nil, err
	}
	setMeta(resp, &out.QueryMeta)
	if out.Sink == nil {
		return nil, CodedError(404, "event sink not found")
	}
	return out.Sink, nil
}

func (s *HTTPServer) eventSinkUpdate(resp http.ResponseWriter, req *http.Request, sinkName string) (interface{}, error) {
	var sink structs.EventSink
	if err := decodeBody(req, &sink); err != nil {
		return nil, CodedError(500, err.Error())
	}

	if sink.ID != sinkName {
		return nil, CodedError(400, "Event sink name does not match request path")
	}

	args := structs.EventSinkUpsertRequest{
		Sink: &sink,
	}
	s.parseWriteRequest(req, &args.WriteRequest)

	var out structs.GenericResponse
	if err := s.agent.RPC("Event.UpsertSink", &args, &out); err != nil {
		return nil, err
	}

	setIndex(resp, out.Index)
	return nil, nil
}

func (s *HTTPServer) eventSinkDelete(resp http.ResponseWriter, req *http.Request, sink string) (interface{}, error) {
	args := structs.EventSinkDeleteRequest{
		IDs: []string{sink},
	}
	s.parseWriteRequest(req, &args.WriteRequest)

	var out structs.GenericResponse
	if err := s.agent.RPC("Event.DeleteSink", &args, &out); err != nil {
		return nil, err
	}
	setIndex(resp, out.Index)
	return nil, nil
}

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

	// Create a goroutine that closes the pipe if the connection closes
	ctx, cancel := context.WithCancel(req.Context())
	defer cancel()
	go func() {
		<-ctx.Done()
		httpPipe.Close()
	}()

	// Create an output that gets flushed on every write
	output := ioutils.NewWriteFlusher(resp)

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
			// Each entry is its own new line according to ndjson.org
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
