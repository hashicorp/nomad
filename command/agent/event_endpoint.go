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
	"github.com/gorilla/websocket"
	"github.com/hashicorp/go-msgpack/codec"
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

	// Uplift to websocket if requested
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
			go forwardEventStreamInput(args, encoder, ws, wsErrCh)
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

			if err := res.Error; err != nil {
				if webSocket {
					code := 500
					if err.Code != nil {
						code = int(*err.Code)
					}
					wsErrCh <- CodedError(code, err.Error())
				}
				if err.Code != nil {
					return CodedError(int(*err.Code), err.Error())
				}
			}

			if webSocket {
				if err := ws.WriteMessage(websocket.TextMessage, res.Event.Data); err != nil {
					codedErr := CodedError(500, err.Error())
					wsErrCh <- codedErr
					return codedErr
				}
			} else {
				// Flush json entry to response
				if _, err := io.Copy(output, bytes.NewReader(res.Event.Data)); err != nil {
					return CodedError(500, err.Error())
				}
				// Each entry is its own new line according to ndjson.org
				// append new line to each entry
				fmt.Fprint(output, "\n")
			}
		}
	})

	// invoke handler
	handler(handlerPipe)
	cancel()

	codedErr := errs.Wait()

	if webSocket {
		// we won't return an error on ws close, but at least make it available in
		// the logs so we can trace spurious disconnects
		if codedErr != nil {
			s.logger.Debug("event stream channel closed with error", "error", codedErr)
		}

		if isClosedError(codedErr) {
			codedErr = nil
		} else if codedErr != nil {
			code := 500
			if _, ok := codedErr.(HTTPCodedError); ok {
				code = codedErr.(HTTPCodedError).Code()
			}
			ws.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(toWsCode(code), codedErr.Error()))
		}
		ws.Close()
	} else {
		if codedErr != nil && strings.Contains(codedErr.Error(), io.ErrClosedPipe.Error()) {
			codedErr = nil
		}
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

// forwardEventStreamInput forwards event stream input from websocket connection
// to the streaming RPC connection at the event publisher.
func forwardEventStreamInput(req *structs.EventStreamRequest, encoder *codec.Encoder, ws *websocket.Conn, errCh chan<- HTTPCodedError) {
	for {
		err := ws.ReadJSON(req)
		if err == io.EOF {
			return
		}

		if err != nil {
			errCh <- CodedError(500, err.Error())
			return
		}

		err = encoder.Encode(req)
		if err != nil {
			errCh <- CodedError(500, err.Error())
		}
	}
}
