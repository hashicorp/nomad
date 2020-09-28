package stream

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

var (
	// NDJsonHeartbeat is the NDJson to send as a heartbeat
	// Avoids creating many heartbeat instances
	NDJsonHeartbeat = &NDJson{Data: []byte("{}\n")}
)

// NDJsonStream is used to send new line delimited JSON and heartbeats
// to a destination (out channel)
type NDJsonStream struct {
	out chan<- *NDJson

	// heartbeat is the interval to send heartbeat messages to keep a connection
	// open.
	heartbeat *time.Ticker

	publishCh chan NDJson
	exitCh    chan struct{}

	l       sync.Mutex
	running bool
}

// NNDJson is a wrapper for a Newline Delimited JSON object
type NDJson struct {
	Data []byte
}

// NewNNewNDJsonStream creates a new NDJson stream that will output NDJson structs
// to the passed output channel
func NewNDJsonStream(out chan<- *NDJson, heartbeat time.Duration) *NDJsonStream {
	return &NDJsonStream{
		out:       out,
		heartbeat: time.NewTicker(heartbeat),
		exitCh:    make(chan struct{}),
		publishCh: make(chan NDJson),
	}
}

// Run starts a long lived goroutine that handles sending
// heartbeats and processed json objects to the streams out channel as well
func (n *NDJsonStream) Run(ctx context.Context) {
	n.l.Lock()
	if n.running {
		return
	}
	n.running = true
	n.l.Unlock()

	go n.run(ctx)
}

func (n *NDJsonStream) run(ctx context.Context) {
	defer func() {
		n.l.Lock()
		n.running = false
		n.l.Unlock()
		close(n.exitCh)
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-n.publishCh:
			n.out <- msg.Copy()
		case <-n.heartbeat.C:
			// Send a heartbeat frame
			select {
			case n.out <- NDJsonHeartbeat:
			case <-ctx.Done():
				return
			}
		}
	}
}

// Send encodes an object into Newline delimited json. An error is returned
// if json encoding fails or if the stream is no longer running.
func (n *NDJsonStream) Send(obj interface{}) error {
	n.l.Lock()
	defer n.l.Unlock()

	buf := bytes.NewBuffer(nil)
	if err := json.NewEncoder(buf).Encode(obj); err != nil {
		return fmt.Errorf("marshaling json for stream: %w", err)
	}

	select {
	case n.publishCh <- NDJson{Data: buf.Bytes()}:
	case <-n.exitCh:
		return fmt.Errorf("stream is no longer running")
	}

	return nil
}

func (j *NDJson) Copy() *NDJson {
	n := new(NDJson)
	*n = *j
	n.Data = make([]byte, len(j.Data))
	copy(n.Data, j.Data)
	return n
}
