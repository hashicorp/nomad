package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mitchellh/mapstructure"
)

const (
	TopicDeployment Topic = "Deployment"
	TopicEvaluation Topic = "Evaluation"
	TopicAllocation Topic = "Allocation"
	TopicJob        Topic = "Job"
	TopicNode       Topic = "Node"
	TopicService    Topic = "Service"
	TopicAll        Topic = "*"
)

// Events is a set of events for a corresponding index. Events returned for the
// index depend on which topics are subscribed to when a request is made.
type Events struct {
	Index  uint64
	Events []Event
	Err    error
}

// Topic is an event Topic
type Topic string

// Event holds information related to an event that occurred in Nomad.
// The Payload is a hydrated object related to the Topic
type Event struct {
	Topic      Topic
	Type       string
	Key        string
	FilterKeys []string
	Index      uint64
	Payload    map[string]interface{}
}

// Deployment returns a Deployment struct from a given event payload. If the
// Event Topic is Deployment this will return a valid Deployment
func (e *Event) Deployment() (*Deployment, error) {
	out, err := e.decodePayload()
	if err != nil {
		return nil, err
	}
	return out.Deployment, nil
}

// Evaluation returns a Evaluation struct from a given event payload. If the
// Event Topic is Evaluation this will return a valid Evaluation
func (e *Event) Evaluation() (*Evaluation, error) {
	out, err := e.decodePayload()
	if err != nil {
		return nil, err
	}
	return out.Evaluation, nil
}

// Allocation returns a Allocation struct from a given event payload. If the
// Event Topic is Allocation this will return a valid Allocation.
func (e *Event) Allocation() (*Allocation, error) {
	out, err := e.decodePayload()
	if err != nil {
		return nil, err
	}
	return out.Allocation, nil
}

// Job returns a Job struct from a given event payload. If the
// Event Topic is Job this will return a valid Job.
func (e *Event) Job() (*Job, error) {
	out, err := e.decodePayload()
	if err != nil {
		return nil, err
	}
	return out.Job, nil
}

// Node returns a Node struct from a given event payload. If the
// Event Topic is Node this will return a valid Node.
func (e *Event) Node() (*Node, error) {
	out, err := e.decodePayload()
	if err != nil {
		return nil, err
	}
	return out.Node, nil
}

// Service returns a ServiceRegistration struct from a given event payload. If
// the Event Topic is Service this will return a valid ServiceRegistration.
func (e *Event) Service() (*ServiceRegistration, error) {
	out, err := e.decodePayload()
	if err != nil {
		return nil, err
	}
	return out.Service, nil
}

type eventPayload struct {
	Allocation *Allocation          `mapstructure:"Allocation"`
	Deployment *Deployment          `mapstructure:"Deployment"`
	Evaluation *Evaluation          `mapstructure:"Evaluation"`
	Job        *Job                 `mapstructure:"Job"`
	Node       *Node                `mapstructure:"Node"`
	Service    *ServiceRegistration `mapstructure:"Service"`
}

func (e *Event) decodePayload() (*eventPayload, error) {
	var out eventPayload
	cfg := &mapstructure.DecoderConfig{
		Result:     &out,
		DecodeHook: mapstructure.StringToTimeHookFunc(time.RFC3339),
	}

	dec, err := mapstructure.NewDecoder(cfg)
	if err != nil {
		return nil, err
	}

	if err := dec.Decode(e.Payload); err != nil {
		return nil, err
	}

	return &out, nil
}

// IsHeartbeat specifies if the event is an empty heartbeat used to
// keep a connection alive.
func (e *Events) IsHeartbeat() bool {
	return e.Index == 0 && len(e.Events) == 0
}

// EventStream is used to stream events from Nomad
type EventStream struct {
	client *Client
}

// EventStream returns a handle to the Events endpoint
func (c *Client) EventStream() *EventStream {
	return &EventStream{client: c}
}

// Stream establishes a new subscription to Nomad's event stream and streams
// results back to the returned channel.
func (e *EventStream) Stream(ctx context.Context, topics map[Topic][]string, index uint64, q *QueryOptions) (<-chan *Events, error) {
	r, err := e.client.newRequest("GET", "/v1/event/stream")
	if err != nil {
		return nil, err
	}
	q = q.WithContext(ctx)
	if q.Params == nil {
		q.Params = map[string]string{}
	}
	q.Params["index"] = strconv.FormatUint(index, 10)
	r.setQueryOptions(q)

	// Build topic query params
	for topic, keys := range topics {
		for _, k := range keys {
			r.params.Add("topic", fmt.Sprintf("%s:%s", topic, k))
		}
	}

	_, resp, err := requireOK(e.client.doRequest(r))

	if err != nil {
		return nil, err
	}

	eventsCh := make(chan *Events, 10)
	go func() {
		defer resp.Body.Close()
		defer close(eventsCh)

		dec := json.NewDecoder(resp.Body)

		for ctx.Err() == nil {
			// Decode next newline delimited json of events
			var events Events
			if err := dec.Decode(&events); err != nil {
				// set error and fallthrough to
				// select eventsCh
				events = Events{Err: err}
			}
			if events.Err == nil && events.IsHeartbeat() {
				continue
			}

			select {
			case <-ctx.Done():
				return
			case eventsCh <- &events:
			}
		}
	}()

	return eventsCh, nil
}

type eventSession struct {
	client  *Client
	alloc   *eventPayload
	task    string
	tty     bool
	command []string

	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer

	terminalSizeCh <-chan TerminalSize

	q *QueryOptions
}

func (s *eventSession) run(ctx context.Context) (exitCode int, err error) {
	ctx, cancelFn := context.WithCancel(ctx)
	defer cancelFn()

	conn, err := s.startConnection()
	if err != nil {
		return -2, err
	}
	defer conn.Close()

	sendErrCh := s.startTransmit(ctx, conn)
	exitCh, recvErrCh := s.startReceiving(ctx, conn)

	for {
		select {
		case <-ctx.Done():
			return -2, ctx.Err()
		case exitCode := <-exitCh:
			return exitCode, nil
		case recvErr := <-recvErrCh:
			// drop websocket code, not relevant to user
			if wsErr, ok := recvErr.(*websocket.CloseError); ok && wsErr.Text != "" {
				return -2, errors.New(wsErr.Text)
			}

			return -2, recvErr
		case sendErr := <-sendErrCh:
			return -2, fmt.Errorf("failed to send input: %w", sendErr)
		}
	}
}

func (s *eventSession) startConnection() (*websocket.Conn, error) {
	// First, attempt to connect to the node directly, but may fail due to network isolation
	// and network errors.  Fallback to using server-side forwarding instead.
	nodeClient, err := s.client.GetNodeClientWithTimeout(s.alloc.NodeID, ClientConnTimeout, s.q)
	if err == NodeDownErr {
		return nil, NodeDownErr
	}

	q := s.q
	if q == nil {
		q = &QueryOptions{}
	}
	if q.Params == nil {
		q.Params = make(map[string]string)
	}

	commandBytes, err := json.Marshal(s.command)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal command: %W", err)
	}

	q.Params["tty"] = strconv.FormatBool(s.tty)
	q.Params["task"] = s.task
	q.Params["command"] = string(commandBytes)

	reqPath := fmt.Sprintf("/v1/client/allocation/%s/exec", s.alloc.ID)

	var conn *websocket.Conn

	if nodeClient != nil {
		conn, _, _ = nodeClient.websocket(reqPath, q)
	}

	if conn == nil {
		conn, _, err = s.client.websocket(reqPath, q)
		if err != nil {
			return nil, err
		}
	}

	return conn, nil
}

func (s *eventSession) startTransmit(ctx context.Context, conn *websocket.Conn) <-chan error {

	// FIXME: Handle websocket send errors.
	// Currently, websocket write failures are dropped. As sending and
	// receiving are running concurrently, it's expected that some send
	// requests may fail with connection errors when connection closes.
	// Connection errors should surface in the receive paths already,
	// but I'm unsure about one-sided communication errors.
	var sendLock sync.Mutex
	send := func(v *ExecStreamingInput) {
		sendLock.Lock()
		defer sendLock.Unlock()

		conn.WriteJSON(v)
	}

	errCh := make(chan error, 4)

	// propagate stdin
	go func() {

		bytes := make([]byte, 2048)
		for {
			if ctx.Err() != nil {
				return
			}

			input := ExecStreamingInput{Stdin: &ExecStreamingIOOperation{}}

			n, err := s.stdin.Read(bytes)

			// always send data if we read some
			if n != 0 {
				input.Stdin.Data = bytes[:n]
				send(&input)
			}

			// then handle error
			if err == io.EOF {
				// if n != 0, send data and we'll get n = 0 on next read
				if n == 0 {
					input.Stdin.Close = true
					send(&input)
					return
				}
			} else if err != nil {
				errCh <- err
				return
			}
		}
	}()

	// propagate terminal sizing updates
	go func() {
		for {
			resizeInput := ExecStreamingInput{}

			select {
			case <-ctx.Done():
				return
			case size, ok := <-s.terminalSizeCh:
				if !ok {
					return
				}
				resizeInput.TTYSize = &size
				send(&resizeInput)
			}

		}
	}()

	// send a heartbeat every 10 seconds
	go func() {
		t := time.NewTimer(heartbeatInterval)
		defer t.Stop()

		for {
			t.Reset(heartbeatInterval)

			select {
			case <-ctx.Done():
				return
			case <-t.C:
				// heartbeat message
				send(&execStreamingInputHeartbeat)
			}
		}
	}()

	return errCh
}

func (s *eventSession) startReceiving(ctx context.Context, conn *websocket.Conn) (<-chan int, <-chan error) {
	exitCodeCh := make(chan int, 1)
	errCh := make(chan error, 1)

	go func() {
		for ctx.Err() == nil {

			// Decode the next frame
			var frame ExecStreamingOutput
			err := conn.ReadJSON(&frame)
			if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				errCh <- fmt.Errorf("websocket closed before receiving exit code: %w", err)
				return
			} else if err != nil {
				errCh <- err
				return
			}

			switch {
			case frame.Stdout != nil:
				if len(frame.Stdout.Data) != 0 {
					s.stdout.Write(frame.Stdout.Data)
				}
				// don't really do anything if stdout is closing
			case frame.Stderr != nil:
				if len(frame.Stderr.Data) != 0 {
					s.stderr.Write(frame.Stderr.Data)
				}
				// don't really do anything if stderr is closing
			case frame.Exited && frame.Result != nil:
				exitCodeCh <- frame.Result.ExitCode
				return
			default:
				// noop - heartbeat
			}

		}

	}()

	return exitCodeCh, errCh
}

// EventStreamingIOOperation represents a stream write operation: either appending data or close (exclusively)
type EventStreamingIOOperation struct {
	Data  []byte `json:"data,omitempty"`
	Close bool   `json:"close,omitempty"`
}

// EventTerminalSize represents the size of the terminal
type EventTerminalSize struct {
	Height int `json:"height,omitempty"`
	Width  int `json:"width,omitempty"`
}

var eventStreamingInputHeartbeat = ExecStreamingInput{}

// EventStreamingInput represents user input to be sent to nomad exec handler.
//
// At most one field should be set.
type EventStreamingInput struct {
	Stdin   *EventStreamingIOOperation `json:"stdin,omitempty"`
	TTYSize *TerminalSize              `json:"tty_size,omitempty"`
}

// EventStreamingExitResult captures the exit code of just completed nomad exec command
type EventStreamingExitResult struct {
	ExitCode int `json:"exit_code"`
}

// EventStreamingOutput represents an output streaming entity, e.g. stdout/stderr update or termination
//
// At most one of these fields should be set: `Stdout`, `Stderr`, or `Result`.
// If `Exited` is true, then `Result` is non-nil, and other fields are nil.
type EventStreamingOutput struct {
	Stdout *EventStreamingIOOperation `json:"stdout,omitempty"`
	Stderr *EventStreamingIOOperation `json:"stderr,omitempty"`

	Exited bool                      `json:"exited,omitempty"`
	Result *EventStreamingExitResult `json:"result,omitempty"`
}
