// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"bytes"
	"context"
	"errors"
	"io"
	"time"

	"github.com/hashicorp/go-msgpack/v2/codec"

	"github.com/hashicorp/nomad/command/agent/host"
	"github.com/hashicorp/nomad/command/agent/monitor"
	"github.com/hashicorp/nomad/command/agent/pprof"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/nomad/structs"

	log "github.com/hashicorp/go-hclog"
	metrics "github.com/hashicorp/go-metrics/compat"
	sframer "github.com/hashicorp/nomad/client/lib/streamframer"
	cstructs "github.com/hashicorp/nomad/client/structs"
)

type Agent struct {
	c *Client
}

func NewAgentEndpoint(c *Client) *Agent {
	a := &Agent{c: c}
	a.c.streamingRpcs.Register("Agent.Monitor", a.monitor)
	a.c.streamingRpcs.Register("Agent.MonitorExternal", a.monitorExternal)
	return a
}

func (a *Agent) Profile(args *structs.AgentPprofRequest, reply *structs.AgentPprofResponse) error {
	// Check ACL for agent write
	aclObj, err := a.c.ResolveToken(args.AuthToken)
	if err != nil {
		return err
	} else if !aclObj.AllowAgentWrite() {
		return structs.ErrPermissionDenied
	}

	if !aclObj.AllowAgentDebug(a.c.GetConfig().EnableDebug) {
		return structs.ErrPermissionDenied
	}

	var resp []byte
	var headers map[string]string

	// Determine which profile to run and generate profile.
	// Blocks for args.Seconds
	// Our RPC endpoints currently don't support context
	// or request cancellation so stubbing with TODO
	switch args.ReqType {
	case pprof.CPUReq:
		resp, headers, err = pprof.CPUProfile(context.TODO(), args.Seconds)
	case pprof.CmdReq:
		resp, headers, err = pprof.Cmdline()
	case pprof.LookupReq:
		resp, headers, err = pprof.Profile(args.Profile, args.Debug, args.GC)
	case pprof.TraceReq:
		resp, headers, err = pprof.Trace(context.TODO(), args.Seconds)
	}

	if err != nil {
		if pprof.IsErrProfileNotFound(err) {
			return structs.NewErrRPCCoded(404, err.Error())
		}
		return structs.NewErrRPCCoded(500, err.Error())
	}

	// Copy profile response to reply
	reply.Payload = resp
	reply.AgentID = a.c.NodeID()
	reply.HTTPHeaders = headers

	return nil
}

func (a *Agent) monitor(conn io.ReadWriteCloser) {
	defer metrics.MeasureSince([]string{"client", "agent", "monitor"}, time.Now())
	defer conn.Close()

	// Decode arguments
	var args cstructs.MonitorRequest
	decoder := codec.NewDecoder(conn, structs.MsgpackHandle)
	encoder := codec.NewEncoder(conn, structs.MsgpackHandle)

	if err := decoder.Decode(&args); err != nil {
		handleStreamResultError(err, pointer.Of(int64(500)), encoder)
		return
	}

	// Check acl
	if aclObj, err := a.c.ResolveToken(args.AuthToken); err != nil {
		handleStreamResultError(err, pointer.Of(int64(403)), encoder)
		return
	} else if !aclObj.AllowAgentRead() {
		handleStreamResultError(structs.ErrPermissionDenied, pointer.Of(int64(403)), encoder)
		return
	}

	logLevel := log.LevelFromString(args.LogLevel)
	if args.LogLevel == "" {
		logLevel = log.LevelFromString("INFO")
	}

	if logLevel == log.NoLevel {
		handleStreamResultError(errors.New("Unknown log level"), pointer.Of(int64(400)), encoder)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	monitor := monitor.New(512, a.c.logger, &log.LoggerOptions{
		JSONFormat:      args.LogJSON,
		Level:           logLevel,
		IncludeLocation: args.LogIncludeLocation,
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
		<-ctx.Done()
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
		handleStreamResultError(streamErr, pointer.Of(int64(500)), encoder)
		return
	}
}

// Host collects data about the host evironment running the agent
func (a *Agent) Host(args *structs.HostDataRequest, reply *structs.HostDataResponse) error {
	aclObj, err := a.c.ResolveToken(args.AuthToken)
	if err != nil {
		return err
	}
	if !aclObj.AllowAgentRead() && !a.c.GetConfig().EnableDebug {
		return structs.ErrPermissionDenied
	}

	data, err := host.MakeHostData()
	if err != nil {
		return err
	}

	reply.AgentID = a.c.NodeID()
	reply.HostData = data
	return nil
}
func (a *Agent) monitorExternal(conn io.ReadWriteCloser) {
	//defer metrics.MeasureSince([]string{"client", "agent", "monitor"}, time.Now())
	defer conn.Close()

	// Decode arguments
	var args cstructs.MonitorExternalRequest
	decoder := codec.NewDecoder(conn, structs.MsgpackHandle)
	encoder := codec.NewEncoder(conn, structs.MsgpackHandle)

	if err := decoder.Decode(&args); err != nil {
		handleStreamResultError(err, pointer.Of(int64(500)), encoder)
		return
	}

	// Check acl
	if aclObj, err := a.c.ResolveToken(args.AuthToken); err != nil {
		handleStreamResultError(err, pointer.Of(int64(403)), encoder)
		return
	} else if !aclObj.AllowAgentRead() {
		handleStreamResultError(structs.ErrPermissionDenied, pointer.Of(int64(403)), encoder)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	monitor := monitor.New(512, a.c.logger, &log.LoggerOptions{})

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
		<-ctx.Done()
	}()
	opts := cstructs.MonitorExternalRequest{
		LogSince:    args.LogSince,
		ServiceName: args.ServiceName,
		Follow:      args.Follow,
		LogPath:     args.LogPath,
	}
	logCh := monitor.MonitorExternal(&opts)

	//defer monitor.Stop()
	initialOffset := int64(0)
	var eofCancelCh chan error
	// receive logs and build frames
	go func() {
		defer framer.Destroy()

		if err := a.streamFixed(ctx, initialOffset, "", 0, logCh, framer, eofCancelCh, true); err != nil {
			select {
			case errCh <- err:
			case <-ctx.Done():
			}
		}
	}()
	var streamErr error
	a.c.logger.Info("before outer loop")
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
				if err := frameCodec.Encode(frame); err != nil && err != io.EOF {
					streamErr = err
					break OUTER
				}
				resp.Payload = buf.Bytes()
				buf.Reset()

			}

			if err := encoder.Encode(resp); err != nil && err != io.EOF {
				streamErr = err
				break OUTER
			}
			encoder.Reset(conn)
		case <-ctx.Done():
			a.c.logger.Info("context was cancelled in send loop")
			break OUTER

		}
	}

	if streamErr != nil {
		handleStreamResultError(streamErr, pointer.Of(int64(500)), encoder)
		return
	}

}

type StreamReader struct {
	ch  <-chan []byte
	buf []byte
}

func NewStreamReader(ch <-chan []byte) *StreamReader {
	return &StreamReader{ch: ch}

}

func (r *StreamReader) Read(p []byte, lg log.Logger) (n int, err error) {
	//if len(r.buf) == 0 {
	select {
	case data, ok := <-r.ch:
		if !ok && len(data) == 0 {
			return 0, io.EOF
		}
		r.buf = data

	default:
		return 0, nil
	}
	//}

	n = copy(p, r.buf)
	r.buf = r.buf[n:]
	return n, nil
}

func (a *Agent) streamFixed(ctx context.Context, offset int64, path string, limit int64,
	channel <-chan []byte, framer *sframer.StreamFramer, eofCancelCh chan error, cancelAfterFirstEof bool) error {
	// streamFrameSize is the maximum number of bytes to send in a single frame
	streamFrameSize := int64(1024)

	bufSize := int64(streamFrameSize)
	if limit > 0 && limit < streamFrameSize {
		bufSize = limit
	}
	streamReader := NewStreamReader(channel)
	streamBuffer := make([]byte, bufSize)

	//// Create a variable to allow setting the last event
	var lastEvent string

	//// Only watch file when there is a need for it
	cancelReceived := cancelAfterFirstEof

OUTER:
	for {

		// Read up to the max frame size
		n, readErr := streamReader.Read(streamBuffer, a.c.logger)

		// Update the offset
		offset += int64(n)

		// Return non-EOF errors
		if readErr != nil && readErr != io.EOF {
			return readErr
		}

		// Send the frame
		if n != 0 || lastEvent != "" {
			if err := framer.Send(path, lastEvent, streamBuffer[:n], offset); err != nil {
				return parseFramerErr(err)
			}
		}

		// Clear the last event
		if lastEvent != "" {
			lastEvent = ""
		}

		// Just keep reading since we aren't at the end of the file so we can
		// avoid setting up a file event watcher.
		if readErr == nil {
			continue
		}

		// At this point we can stop without waiting for more changes,
		// because we have EOF and either we're not following at all,
		// or we received an event from the eofCancelCh channel
		// and last read was executed
		if cancelReceived {
			return nil
		}

		for {
			select {
			case <-framer.ExitCh():
				a.c.logger.Info("hit the framer exit chan")
				return nil
			case <-ctx.Done():
				a.c.logger.Info("hit context.done ")
				return nil
			case _, ok := <-eofCancelCh:
				a.c.logger.Info("hit the eof cancel chan")
				if !ok {
					return nil
				}
				cancelReceived = true
				continue OUTER
			}
		}
	}
}
