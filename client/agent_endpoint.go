// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"bytes"
	"context"
	"errors"
	"io"
	"time"

	"github.com/hashicorp/go-msgpack/codec"

	"github.com/hashicorp/nomad/command/agent/host"
	"github.com/hashicorp/nomad/command/agent/monitor"
	"github.com/hashicorp/nomad/command/agent/pprof"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/nomad/structs"

	metrics "github.com/armon/go-metrics"
	log "github.com/hashicorp/go-hclog"

	sframer "github.com/hashicorp/nomad/client/lib/streamframer"
	cstructs "github.com/hashicorp/nomad/client/structs"
)

type Agent struct {
	c *Client
}

func NewAgentEndpoint(c *Client) *Agent {
	a := &Agent{c: c}
	a.c.streamingRpcs.Register("Agent.Monitor", a.monitor)
	return a
}

func (a *Agent) Profile(args *structs.AgentPprofRequest, reply *structs.AgentPprofResponse) error {
	// Check ACL for agent write
	aclObj, err := a.c.ResolveToken(args.AuthToken)
	if err != nil {
		return err
	} else if aclObj != nil && !aclObj.AllowAgentWrite() {
		return structs.ErrPermissionDenied
	}

	// If ACLs are disabled, EnableDebug must be enabled
	if aclObj == nil && !a.c.GetConfig().EnableDebug {
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
	} else if aclObj != nil && !aclObj.AllowAgentRead() {
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
	if (aclObj != nil && !aclObj.AllowAgentRead()) ||
		(aclObj == nil && !a.c.GetConfig().EnableDebug) {
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
