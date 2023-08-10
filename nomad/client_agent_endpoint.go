// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	log "github.com/hashicorp/go-hclog"

	sframer "github.com/hashicorp/nomad/client/lib/streamframer"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/command/agent/host"
	"github.com/hashicorp/nomad/command/agent/monitor"
	"github.com/hashicorp/nomad/command/agent/pprof"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/nomad/structs"

	"github.com/hashicorp/go-msgpack/codec"
)

type Agent struct {
	srv *Server
}

func NewAgentEndpoint(srv *Server) *Agent {
	return &Agent{srv: srv}
}

func (a *Agent) register() {
	a.srv.streamingRpcs.Register("Agent.Monitor", a.monitor)
}

func (a *Agent) Profile(args *structs.AgentPprofRequest, reply *structs.AgentPprofResponse) error {
	authErr := a.srv.Authenticate(nil, args)
	a.srv.MeasureRPCRate("agent", structs.RateMetricRead, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}

	// Check ACL for agent write
	aclObj, err := a.srv.ResolveACL(args)
	if err != nil {
		return err
	} else if aclObj != nil && !aclObj.AllowAgentWrite() {
		return structs.ErrPermissionDenied
	}

	// Forward to different region if necessary
	// this would typically be done in a.srv.forward() but since
	// we are targeting a specific server, not just the leader
	// we must manually handle region forwarding here.
	region := args.RequestRegion()
	if region == "" {
		return fmt.Errorf("missing target RPC")
	}

	if region != a.srv.config.Region {
		// Mark that we are forwarding
		args.SetForwarded()
		return a.srv.forwardRegion(region, "Agent.Profile", args, reply)
	}

	// Targeting a node, forward request to node
	if args.NodeID != "" {
		return a.forwardProfileClient(args, reply)
	}

	// Handle serverID not equal to ours
	if args.ServerID != "" {
		serverToFwd, err := a.forwardFor(args.ServerID, region)
		if err != nil {
			return err
		}
		if serverToFwd != nil {
			return a.srv.forwardServer(serverToFwd, "Agent.Profile", args, reply)
		}
	}

	// If ACLs are disabled, EnableDebug must be enabled
	if aclObj == nil && !a.srv.config.EnableDebug {
		return structs.ErrPermissionDenied
	}

	// Process the request on this server
	var resp []byte
	var headers map[string]string

	// Determine which profile to run and generate profile.
	// Blocks for args.Seconds
	// Our RPC endpoints currently don't support context
	// or request cancellation so using server shutdownCtx as a
	// best effort.
	switch args.ReqType {
	case pprof.CPUReq:
		resp, headers, err = pprof.CPUProfile(a.srv.shutdownCtx, args.Seconds)
	case pprof.CmdReq:
		resp, headers, err = pprof.Cmdline()
	case pprof.LookupReq:
		resp, headers, err = pprof.Profile(args.Profile, args.Debug, args.GC)
	case pprof.TraceReq:
		resp, headers, err = pprof.Trace(a.srv.shutdownCtx, args.Seconds)
	default:
		err = structs.NewErrRPCCoded(404, "Unknown profile request type")
	}

	if err != nil {
		if pprof.IsErrProfileNotFound(err) {
			return structs.NewErrRPCCoded(404, err.Error())
		}
		return structs.NewErrRPCCoded(500, err.Error())
	}

	// Copy profile response to reply
	reply.Payload = resp
	reply.HTTPHeaders = headers
	reply.AgentID = a.srv.serf.LocalMember().Name

	return nil
}

func (a *Agent) monitor(conn io.ReadWriteCloser) {
	defer conn.Close()

	// Decode args
	var args cstructs.MonitorRequest
	decoder := codec.NewDecoder(conn, structs.MsgpackHandle)
	encoder := codec.NewEncoder(conn, structs.MsgpackHandle)

	if err := decoder.Decode(&args); err != nil {
		handleStreamResultError(err, pointer.Of(int64(500)), encoder)
		return
	}
	authErr := a.srv.Authenticate(nil, &args)
	a.srv.MeasureRPCRate("agent", structs.RateMetricRead, &args)
	if authErr != nil {
		handleStreamResultError(structs.ErrPermissionDenied, nil, encoder)
		return
	}

	// Check agent read permissions
	if aclObj, err := a.srv.ResolveACL(&args); err != nil {
		handleStreamResultError(err, nil, encoder)
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

	// Targeting a node, forward request to node
	if args.NodeID != "" {
		a.forwardMonitorClient(conn, args, encoder, decoder)
		// forwarded request has ended, return
		return
	}

	region := args.RequestRegion()
	if region == "" {
		handleStreamResultError(fmt.Errorf("missing target RPC"), pointer.Of(int64(400)), encoder)
		return
	}
	if region != a.srv.config.Region {
		// Mark that we are forwarding
		args.SetForwarded()
	}

	// Try to forward request to remote region/server
	if args.ServerID != "" {
		serverToFwd, err := a.forwardFor(args.ServerID, region)
		if err != nil {
			handleStreamResultError(err, pointer.Of(int64(400)), encoder)
			return
		}
		if serverToFwd != nil {
			a.forwardMonitorServer(conn, serverToFwd, args, encoder, decoder)
			return
		}
	}

	// NodeID was empty, ServerID was equal to this server,  monitor this server
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	monitor := monitor.New(512, a.srv.logger, &log.LoggerOptions{
		Level:      logLevel,
		JSONFormat: args.LogJSON,
	})

	frames := make(chan *sframer.StreamFrame, 32)
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

// forwardFor returns a serverParts for a request to be forwarded to.
// A response of nil, nil indicates that the current server is equal to the
// serverID and region so the request should not be forwarded.
func (a *Agent) forwardFor(serverID, region string) (*serverParts, error) {
	var target *serverParts
	var err error

	if serverID == "leader" {
		isLeader, remoteLeader := a.srv.getLeader()
		if !isLeader && remoteLeader != nil {
			target = remoteLeader
		} else if !isLeader && remoteLeader == nil {
			return nil, structs.ErrNoLeader
		} else if isLeader {
			// This server is current leader do not forward
			return nil, nil
		}
	} else {
		target, err = a.srv.getServer(region, serverID)
		if err != nil {
			return nil, err
		}
	}

	// Unable to find a server
	if target == nil {
		return nil, fmt.Errorf("unknown nomad server %s", serverID)
	}

	// ServerID is this current server,
	// No need to forward request
	if target.Name == a.srv.LocalMember().Name {
		return nil, nil
	}

	return target, nil
}

func (a *Agent) forwardMonitorClient(conn io.ReadWriteCloser, args cstructs.MonitorRequest, encoder *codec.Encoder, decoder *codec.Decoder) {
	// Get the Connection to the client either by fowarding to another server
	// or creating direct stream

	state, srv, err := a.findClientConn(args.NodeID)
	if err != nil {
		handleStreamResultError(err, pointer.Of(int64(500)), encoder)
		return
	}

	var clientConn net.Conn

	if state == nil {
		conn, err := a.srv.streamingRpc(srv, "Agent.Monitor")
		if err != nil {
			handleStreamResultError(err, nil, encoder)
			return
		}

		clientConn = conn
	} else {
		stream, err := NodeStreamingRpc(state.Session, "Agent.Monitor")
		if err != nil {
			handleStreamResultError(err, nil, encoder)
			return
		}
		clientConn = stream
	}
	defer clientConn.Close()

	// Send the Request
	outEncoder := codec.NewEncoder(clientConn, structs.MsgpackHandle)
	if err := outEncoder.Encode(args); err != nil {
		handleStreamResultError(err, nil, encoder)
		return
	}

	structs.Bridge(conn, clientConn)
}

func (a *Agent) forwardMonitorServer(conn io.ReadWriteCloser, server *serverParts, args cstructs.MonitorRequest, encoder *codec.Encoder, decoder *codec.Decoder) {
	// empty ServerID to prevent forwarding loop
	args.ServerID = ""

	serverConn, err := a.srv.streamingRpc(server, "Agent.Monitor")
	if err != nil {
		handleStreamResultError(err, pointer.Of(int64(500)), encoder)
		return
	}
	defer serverConn.Close()

	// Send the Request
	outEncoder := codec.NewEncoder(serverConn, structs.MsgpackHandle)
	if err := outEncoder.Encode(args); err != nil {
		handleStreamResultError(err, pointer.Of(int64(500)), encoder)
		return
	}

	structs.Bridge(conn, serverConn)
}

func (a *Agent) forwardProfileClient(args *structs.AgentPprofRequest, reply *structs.AgentPprofResponse) error {
	state, srv, err := a.findClientConn(args.NodeID)

	if err != nil {
		return err
	}

	if srv != nil {
		return a.srv.forwardServer(srv, "Agent.Profile", args, reply)
	}

	// NodeRpc
	rpcErr := NodeRpc(state.Session, "Agent.Profile", args, reply)
	if rpcErr != nil {
		return rpcErr
	}

	return nil
}

// Host returns data about the agent's host system for the `debug` command.
func (a *Agent) Host(args *structs.HostDataRequest, reply *structs.HostDataResponse) error {
	authErr := a.srv.Authenticate(nil, args)
	a.srv.MeasureRPCRate("agent", structs.RateMetricRead, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	aclObj, err := a.srv.ResolveACL(args)
	if err != nil {
		return err
	}
	if (aclObj != nil && !aclObj.AllowAgentRead()) ||
		(aclObj == nil && !a.srv.config.EnableDebug) {
		return structs.ErrPermissionDenied
	}

	// Forward to different region if necessary
	// this would typically be done in a.srv.forward() but since
	// we are targeting a specific server, not just the leader
	// we must manually handle region forwarding here.
	region := args.RequestRegion()
	if region == "" {
		return fmt.Errorf("missing target RPC")
	}

	if region != a.srv.config.Region {
		// Mark that we are forwarding
		args.SetForwarded()
		return a.srv.forwardRegion(region, "Agent.Host", args, reply)
	}

	// Targeting a client node, forward request to node
	if args.NodeID != "" {
		client, srv, err := a.findClientConn(args.NodeID)

		if err != nil {
			return err
		}

		if srv != nil {
			return a.srv.forwardServer(srv, "Agent.Host", args, reply)
		}

		return NodeRpc(client.Session, "Agent.Host", args, reply)
	}

	// Handle serverID not equal to ours
	if args.ServerID != "" {
		srv, err := a.forwardFor(args.ServerID, region)
		if err != nil {
			return err
		}
		if srv != nil {
			return a.srv.forwardServer(srv, "Agent.Host", args, reply)
		}
	}

	data, err := host.MakeHostData()
	if err != nil {
		return err
	}

	reply.AgentID = a.srv.serf.LocalMember().Name
	reply.HostData = data
	return nil
}

// findClientConn is a helper that returns a connection to the client node or, if the client
// is connected to a different server, a serverParts describing the server to which the
// client bound RPC should be forwarded.
func (a *Agent) findClientConn(nodeID string) (*nodeConnState, *serverParts, error) {
	snap, err := a.srv.State().Snapshot()
	if err != nil {
		return nil, nil, structs.NewErrRPCCoded(500, err.Error())
	}

	node, err := snap.NodeByID(nil, nodeID)
	if err != nil {
		return nil, nil, structs.NewErrRPCCoded(500, err.Error())
	}

	if node == nil {
		err := fmt.Errorf("Unknown node %q", nodeID)
		return nil, nil, structs.NewErrRPCCoded(404, err.Error())
	}

	if err := nodeSupportsRpc(node); err != nil {
		return nil, nil, structs.NewErrRPCCoded(400, err.Error())
	}

	// Get the Connection to the client either by fowarding to another server
	// or creating direct stream
	state, ok := a.srv.getNodeConn(nodeID)
	if ok {
		return state, nil, nil
	}

	// Determine the server that has a connection to the node
	srv, err := a.srv.serverWithNodeConn(nodeID, a.srv.Region())
	if err != nil {
		code := 500
		if structs.IsErrNoNodeConn(err) {
			code = 404
		}
		return nil, nil, structs.NewErrRPCCoded(code, err.Error())
	}

	return nil, srv, nil
}
