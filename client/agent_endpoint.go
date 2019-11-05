package client

import (
	"bytes"
	"context"
	"errors"
	"io"
	"time"

	"github.com/hashicorp/nomad/command/agent/monitor"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/ugorji/go/codec"

	metrics "github.com/armon/go-metrics"
	log "github.com/hashicorp/go-hclog"
	sframer "github.com/hashicorp/nomad/client/lib/streamframer"
	cstructs "github.com/hashicorp/nomad/client/structs"
)

type Agent struct {
	c *Client
}

func NewAgentEndpoint(c *Client) *Agent {
	m := &Agent{c: c}
	m.c.streamingRpcs.Register("Agent.Monitor", m.monitor)
	return m
}

func (m *Agent) monitor(conn io.ReadWriteCloser) {
	defer metrics.MeasureSince([]string{"client", "agent", "monitor"}, time.Now())
	defer conn.Close()

	// Decode arguments
	var args cstructs.MonitorRequest
	decoder := codec.NewDecoder(conn, structs.MsgpackHandle)
	encoder := codec.NewEncoder(conn, structs.MsgpackHandle)

	if err := decoder.Decode(&args); err != nil {
		handleStreamResultError(err, helper.Int64ToPtr(500), encoder)
		return
	}

	// Check acl
	if aclObj, err := m.c.ResolveToken(args.AuthToken); err != nil {
		handleStreamResultError(err, helper.Int64ToPtr(403), encoder)
		return
	} else if aclObj != nil && !aclObj.AllowAgentRead() {
		handleStreamResultError(structs.ErrPermissionDenied, helper.Int64ToPtr(403), encoder)
		return
	}

	logLevel := log.LevelFromString(args.LogLevel)
	if args.LogLevel == "" {
		logLevel = log.LevelFromString("INFO")
	}

	if logLevel == log.NoLevel {
		handleStreamResultError(errors.New("Unknown log level"), helper.Int64ToPtr(400), encoder)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	monitor := monitor.New(512, m.c.logger, &log.LoggerOptions{
		JSONFormat: args.LogJSON,
		Level:      logLevel,
	})

	frames := make(chan *sframer.StreamFrame, streamFramesBuffer)
	errCh := make(chan error)
	var buf bytes.Buffer
	frameCodec := codec.NewEncoder(&buf, structs.JsonHandle)

	framer := sframer.NewStreamFramer(frames, 1*time.Second, 200*time.Millisecond, 1024)
	framer.Run()
	defer framer.Destroy()

	// goroutine to detect remote side closing
	go func() {
		if _, err := conn.Read(nil); err != nil {
			// One end of the pipe explicitly closed, exit
			cancel()
			return
		}
		select {
		case <-ctx.Done():
			return
		}
	}()

	logCh := monitor.Start()
	defer monitor.Stop()
	initialOffset := int64(0)

	// receive logs and build frames
	go func() {
		defer framer.Destroy()
	LOOP:
		for {
			select {
			case log := <-logCh:
				if err := framer.Send("", "log", log, initialOffset); err != nil {
					select {
					case errCh <- err:
					case <-ctx.Done():
					}
					break LOOP
				}
			case <-ctx.Done():
				break LOOP
			}
		}
	}()

	var streamErr error
OUTER:
	for {
		select {
		case frame, ok := <-frames:
			if !ok {
				// frame may have been closed when an error
				// occurred. Check once more for an error.
				select {
				case streamErr = <-errCh:
					// There was a pending error!
				default:
					// No error, continue on
				}

				break OUTER
			}

			var resp cstructs.StreamErrWrapper
			if args.PlainText {
				resp.Payload = frame.Data
			} else {
				if err := frameCodec.Encode(frame); err != nil {
					streamErr = err
					break OUTER
				}

				resp.Payload = buf.Bytes()
				buf.Reset()
			}

			if err := encoder.Encode(resp); err != nil {
				streamErr = err
				break OUTER
			}
			encoder.Reset(conn)
		case <-ctx.Done():
			break OUTER
		}
	}

	if streamErr != nil {
		handleStreamResultError(streamErr, helper.Int64ToPtr(500), encoder)
		return
	}
}
