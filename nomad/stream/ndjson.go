// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package stream

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-msgpack/v2/codec"

	"github.com/hashicorp/nomad/nomad/structs"
)

var (
	// JsonHeartbeat is an empty JSON object to send as a heartbeat
	// Avoids creating many heartbeat instances
	JsonHeartbeat = &structs.EventJson{Data: []byte("{}")}

	// jsonSendTimeout is the maximum time Send will block trying to enqueue a
	// message to the outbound channel before returning an error. This prevents
	// producers from blocking indefinitely when the consumer is backpressured.
	jsonSendTimeout = 5 * time.Second

	// jsonWarnBlockThreshold is the duration above which we'll emit a simple
	// warning about blocked sends. Keep this small to catch transient stalls.
	jsonWarnBlockThreshold = 200 * time.Millisecond
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

	logger hclog.Logger
}

// NewJsonStream creates a new json stream that will output Json structs
// to the passed output channel. The constructor starts a goroutine
// to begin heartbeating on its set interval and also sends an initial heartbeat
// to notify the client about the successful connection initialization.
func NewJsonStream(ctx context.Context, heartbeat time.Duration, logger hclog.Logger) *JsonStream {
	s := &JsonStream{
		ctx:           ctx,
		outCh:         make(chan *structs.EventJson, 10),
		heartbeatTick: time.NewTicker(heartbeat),
		logger:        logger,
	}

	// Use pushOut so the initial heartbeat benefits from the same backpressure
	// handling / instrumentation as normal sends.
	_ = s.pushOut(JsonHeartbeat)
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
			// Send a heartbeat frame. If pushOut fails due to context cancel or
			// timeout, terminate the heartbeat loop as the stream is no longer
			// healthy.
			if err := n.pushOut(JsonHeartbeat); err != nil {
				// If context is done, just return. Otherwise log a message for
				// debugging.
				if n.ctx.Err() != nil {
					return
				}
				n.logger.Warn("heartbeat push failed", "error", err)
				return
			}
		}
	}
}

// pushOut attempts to enqueue a message into the out channel without blocking
// forever. It will try an immediate, non-blocking enqueue first, and then wait
// up to jsonSendTimeout. If the context is cancelled before the message can be
// enqueued, it returns the context error.
func (n *JsonStream) pushOut(msg *structs.EventJson) error {
	start := time.Now()

	// Fast-path: try immediate non-blocking send
	select {
	case n.outCh <- msg:
		elapsed := time.Since(start)
		if elapsed > jsonWarnBlockThreshold {
			n.logger.Warn("json push experienced short block", "duration", elapsed)
		}
		return nil
	default:
	}

	// If immediate send failed, wait with timeout so we don't block forever.
	timer := time.NewTimer(jsonSendTimeout)
	defer timer.Stop()

	select {
	case n.outCh <- msg:
		elapsed := time.Since(start)
		if elapsed > jsonWarnBlockThreshold {
			n.logger.Warn("json push blocked", "duration", elapsed)
		}
		return nil
	case <-n.ctx.Done():
		return n.ctx.Err()
	case <-timer.C:
		return fmt.Errorf("timed out sending event to stream after %s", jsonSendTimeout)
	}
}

// Send encodes an object into Newline delimited json. An error is returned
// if json encoding fails or if the stream is no longer running.
func (n *JsonStream) Send(v interface{}) error {
	if n.ctx.Err() != nil {
		return n.ctx.Err()
	}

	var buf bytes.Buffer
	enc := codec.NewEncoder(&buf, structs.JsonHandleWithExtensions)
	err := enc.Encode(v)
	if err != nil {
		return fmt.Errorf("error marshaling json for stream: %w", err)
	}

	// pushOut enforces a timeout
	if err := n.pushOut(&structs.EventJson{Data: buf.Bytes()}); err != nil {
		return fmt.Errorf("error sending to json stream: %w", err)
	}

	return nil
}
