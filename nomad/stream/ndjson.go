package stream

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hashicorp/nomad/nomad/structs"
)

var (
	// JsonHeartbeat is an empty JSON object to send as a heartbeat
	// Avoids creating many heartbeat instances
	JsonHeartbeat = &structs.EventJson{Data: []byte("{}")}
)

// JsonStream is used to send new line delimited JSON and heartbeats
// to a destination (out channel)
type JsonStream struct {
	// ctx is a passed in context used to notify the json stream
	// when it should terminate
	ctx context.Context

	outCh chan *structs.EventJson

	// heartbeat is the interval to send heartbeat messages to keep a connection
	// open.
	heartbeatTick *time.Ticker
}

// NewJsonStream creates a new json stream that will output Json structs
// to the passed output channel. The constructor starts a goroutine
// to begin heartbeating on its set interval.
func NewJsonStream(ctx context.Context, heartbeat time.Duration) *JsonStream {
	s := &JsonStream{
		ctx:           ctx,
		outCh:         make(chan *structs.EventJson, 10),
		heartbeatTick: time.NewTicker(heartbeat),
	}

	go s.heartbeat()

	return s
}

func (n *JsonStream) OutCh() chan *structs.EventJson {
	return n.outCh
}

func (n *JsonStream) heartbeat() {
	for {
		select {
		case <-n.ctx.Done():
			return
		case <-n.heartbeatTick.C:
			// Send a heartbeat frame
			select {
			case n.outCh <- JsonHeartbeat:
			case <-n.ctx.Done():
				return
			}
		}
	}
}

// Send encodes an object into Newline delimited json. An error is returned
// if json encoding fails or if the stream is no longer running.
func (n *JsonStream) Send(v interface{}) error {
	if n.ctx.Err() != nil {
		return n.ctx.Err()
	}

	buf, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("error marshaling json for stream: %w", err)
	}

	select {
	case <-n.ctx.Done():
		return fmt.Errorf("error stream is no longer running: %w", err)
	case n.outCh <- &structs.EventJson{Data: buf}:
	}

	return nil
}
