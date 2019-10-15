package client

import (
	"context"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/command/agent/monitor"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/ugorji/go/codec"

	metrics "github.com/armon/go-metrics"
	log "github.com/hashicorp/go-hclog"
	cstructs "github.com/hashicorp/nomad/client/structs"
)

type Monitor struct {
	c *Client
}

func NewMonitorEndpoint(c *Client) *Monitor {
	m := &Monitor{c: c}
	m.c.streamingRpcs.Register("Agent.Monitor", m.monitor)
	return m
}

func (m *Monitor) monitor(conn io.ReadWriteCloser) {
	defer metrics.MeasureSince([]string{"client", "monitor", "monitor"}, time.Now())
	defer conn.Close()

	// Decode arguments
	var req cstructs.MonitorRequest
	decoder := codec.NewDecoder(conn, structs.MsgpackHandle)
	encoder := codec.NewEncoder(conn, structs.MsgpackHandle)

	if err := decoder.Decode(&req); err != nil {
		handleStreamResultError(err, helper.Int64ToPtr(500), encoder)
		return
	}

	// Check acl
	if aclObj, err := m.c.ResolveToken(req.QueryOptions.AuthToken); err != nil {
		handleStreamResultError(err, helper.Int64ToPtr(403), encoder)
		return
	} else if aclObj != nil && !aclObj.AllowNsOp(req.Namespace, acl.NamespaceCapabilityReadFS) {
		handleStreamResultError(structs.ErrPermissionDenied, helper.Int64ToPtr(403), encoder)
		return
	}

	var logLevel log.Level
	if req.LogLevel == "" {
		logLevel = log.LevelFromString("INFO")
	} else {
		logLevel = log.LevelFromString(req.LogLevel)
	}

	if logLevel == log.NoLevel {
		handleStreamResultError(errors.New("Unknown log level"), helper.Int64ToPtr(400), encoder)
		return
	}

	stopCh := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer close(stopCh)
	defer cancel()

	monitor := monitor.New(512, m.c.logger, &log.LoggerOptions{
		Level:      logLevel,
		JSONFormat: false,
	})

	go func() {
		if _, err := conn.Read(nil); err != nil {
			close(stopCh)
			cancel()
			return
		}
		select {
		case <-ctx.Done():
			return
		}
	}()

	logCh := monitor.Start(stopCh)

	var streamErr error
OUTER:
	for {
		select {
		case log := <-logCh:
			var resp cstructs.StreamErrWrapper
			resp.Payload = log
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

		// Attempt to send the error
		encoder.Encode(&cstructs.StreamErrWrapper{
			Error: cstructs.NewRpcError(streamErr, helper.Int64ToPtr(500)),
		})
		return
	}
}
