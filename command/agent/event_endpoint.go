package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/docker/docker/pkg/ioutils"
	"github.com/gorilla/websocket"
	"github.com/hashicorp/go-msgpack/codec"
	cstructs "github.com/hashicorp/nomad/client/structs"
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

	// Uplift to websocket if requestes
	webSocket := query.Has("web_socket")
	var ws *websocket.Conn
	var wsErr error
	var wsErrCh chan HTTPCodedError
	if webSocket {
		ws, wsErr = s.wsUpgrader.Upgrade(resp, req, nil)
		if wsErr != nil {
			return nil, fmt.Errorf("failed to upgrade connection: %v", err)
		}

		if err := readWsHandshake(ws.ReadJSON, req, &args.QueryOptions); err != nil {
			ws.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(toWsCode(400), err.Error()))
			return nil, err
		}

		// Create a channel that decodes the results
		wsErrCh = make(chan HTTPCodedError, 2)
	}

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

		if ws != nil {
			go forwardEventStreamInput(encoder, ws, wsErrCh)
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
				if webSocket && isClosedError(err) {
					ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
					wsErrCh <- nil
				}
				return CodedError(500, err.Error())
			}
			decoder.Reset(httpPipe)

			LEFT OFF HERE 
			
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

func (s *HTTPServer) EventStreamWebSocket(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Build the request and parse the ACL token
	task := req.URL.Query().Get("task")
	cmdJsonStr := req.URL.Query().Get("command")
	var command []string
	err := json.Unmarshal([]byte(cmdJsonStr), &command)
	if err != nil {
		// this shouldn't happen, []string is always be serializable to json
		return nil, fmt.Errorf("failed to marshal command into json: %v", err)
	}

	ttyB := false
	if tty := req.URL.Query().Get("tty"); tty != "" {
		ttyB, err = strconv.ParseBool(tty)
		if err != nil {
			return nil, fmt.Errorf("tty value is not a boolean: %v", err)
		}
	}

	args := cstructs.AllocExecRequest{
		AllocID: allocID,
		Task:    task,
		Cmd:     command,
		Tty:     ttyB,
	}
	s.parse(resp, req, &args.QueryOptions.Region, &args.QueryOptions)

	conn, err := s.wsUpgrader.Upgrade(resp, req, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to upgrade connection: %v", err)
	}

	if err := readWsHandshake(conn.ReadJSON, req, &args.QueryOptions); err != nil {
		conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(toWsCode(400), err.Error()))
		return nil, err
	}

	return s.eventStreamWebSocket(conn, &args)
}

func (s *HTTPServer) eventStreamWebSocket(ws *websocket.Conn, args *cstructs.AllocExecRequest) (interface{}, error) {
	allocID := args.AllocID
	method := "Allocations.Exec"

	// Get the correct handler
	localClient, remoteClient, localServer := s.rpcHandlerForAlloc(allocID)
	var handler structs.StreamingRpcHandler
	var handlerErr error
	if localClient {
		handler, handlerErr = s.agent.Client().StreamingRpcHandler(method)
	} else if remoteClient {
		handler, handlerErr = s.agent.Client().RemoteStreamingRpcHandler(method)
	} else if localServer {
		handler, handlerErr = s.agent.Server().StreamingRpcHandler(method)
	}

	if handlerErr != nil {
		return nil, CodedError(500, handlerErr.Error())
	}

	// Create a pipe connecting the (possibly remote) handler to the http response
	httpPipe, handlerPipe := net.Pipe()
	decoder := codec.NewDecoder(httpPipe, structs.MsgpackHandle)
	encoder := codec.NewEncoder(httpPipe, structs.MsgpackHandle)

	// Create a goroutine that closes the pipe if the connection closes.
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-ctx.Done()
		httpPipe.Close()

		// don't close ws - wait to drain messages
	}()

	// Create a channel that decodes the results
	errCh := make(chan HTTPCodedError, 2)

	// stream response
	go func() {
		defer cancel()

		// Send the request
		if err := encoder.Encode(args); err != nil {
			errCh <- CodedError(500, err.Error())
			return
		}

		go forwardExecInput(encoder, ws, errCh)

		for {
			var res cstructs.StreamErrWrapper
			err := decoder.Decode(&res)
			if isClosedError(err) {
				ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				errCh <- nil
				return
			}

			if err != nil {
				errCh <- CodedError(500, err.Error())
				return
			}
			decoder.Reset(httpPipe)

			if err := res.Error; err != nil {
				code := 500
				if err.Code != nil {
					code = int(*err.Code)
				}
				errCh <- CodedError(code, err.Error())
				return
			}

			if err := ws.WriteMessage(websocket.TextMessage, res.Payload); err != nil {
				errCh <- CodedError(500, err.Error())
				return
			}
		}
	}()

	// start streaming request to streaming RPC - returns when streaming completes or errors
	handler(handlerPipe)
	// stop streaming background goroutines for streaming - but not websocket activity
	cancel()
	// retrieve any error and/or wait until goroutine stop and close errCh connection before
	// closing websocket connection
	codedErr := <-errCh

	// we won't return an error on ws close, but at least make it available in
	// the logs so we can trace spurious disconnects
	if codedErr != nil {
		s.logger.Debug("alloc exec channel closed with error", "error", codedErr)
	}

	if isClosedError(codedErr) {
		codedErr = nil
	} else if codedErr != nil {
		ws.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(toWsCode(codedErr.Code()), codedErr.Error()))
	}
	ws.Close()

	return nil, codedErr
}
