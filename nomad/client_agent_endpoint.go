package nomad

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	log "github.com/hashicorp/go-hclog"
	sframer "github.com/hashicorp/nomad/client/lib/streamframer"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/command/agent/monitor"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"

	"github.com/ugorji/go/codec"
)

type Agent struct {
	srv *Server
}

func (m *Agent) register() {
	m.srv.streamingRpcs.Register("Agent.Monitor", m.monitor)
}

func (m *Agent) monitor(conn io.ReadWriteCloser) {
	defer conn.Close()

	// Decode args
	var args cstructs.MonitorRequest
	decoder := codec.NewDecoder(conn, structs.MsgpackHandle)
	encoder := codec.NewEncoder(conn, structs.MsgpackHandle)

	if err := decoder.Decode(&args); err != nil {
		handleStreamResultError(err, helper.Int64ToPtr(500), encoder)
		return
	}

	// Check node read permissions
	if aclObj, err := m.srv.ResolveToken(args.AuthToken); err != nil {
		handleStreamResultError(err, nil, encoder)
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

	// Targeting a client so forward the request
	if args.NodeID != "" {
		nodeID := args.NodeID

		snap, err := m.srv.State().Snapshot()
		if err != nil {
			handleStreamResultError(err, nil, encoder)
			return
		}

		node, err := snap.NodeByID(nil, nodeID)
		if err != nil {
			handleStreamResultError(err, helper.Int64ToPtr(500), encoder)
			return
		}

		if node == nil {
			err := fmt.Errorf("Unknown node %q", nodeID)
			handleStreamResultError(err, helper.Int64ToPtr(400), encoder)
			return
		}

		if err := nodeSupportsRpc(node); err != nil {
			handleStreamResultError(err, helper.Int64ToPtr(400), encoder)
			return
		}

		// Get the Connection to the client either by fowarding to another server
		// or creating direct stream
		var clientConn net.Conn
		state, ok := m.srv.getNodeConn(nodeID)
		if !ok {
			// Determine the server that has a connection to the node
			srv, err := m.srv.serverWithNodeConn(nodeID, m.srv.Region())
			if err != nil {
				var code *int64
				if structs.IsErrNoNodeConn(err) {
					code = helper.Int64ToPtr(404)
				}
				handleStreamResultError(err, code, encoder)
				return
			}
			conn, err := m.srv.streamingRpc(srv, "Agent.Monitor")
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
		return
	}

	// NodeID was empty, so monitor this current server
	stopCh := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer close(stopCh)
	defer cancel()

	monitor := monitor.New(512, m.srv.logger, &log.LoggerOptions{
		Level:      logLevel,
		JSONFormat: args.LogJSON,
	})

	frames := make(chan *sframer.StreamFrame, 32)
	errCh := make(chan error)
	var buf bytes.Buffer
	frameCodec := codec.NewEncoder(&buf, structs.JsonHandle)

	// framer := sframer.NewStreamFramer(frames, 1*time.Second, 200*time.Millisecond, 64*1024)
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

	logCh := monitor.Start(stopCh)
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
		// Nothing to do as conn is closed
		if streamErr == io.EOF || strings.Contains(streamErr.Error(), "closed") {
			return
		}
	}
}
