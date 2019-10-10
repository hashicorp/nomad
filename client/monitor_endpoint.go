package client

import (
	"context"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/hashicorp/nomad/acl"
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
	m.c.streamingRpcs.Register("Client.Monitor", m.monitor)
	return m
}

func (m *Monitor) monitor(conn io.ReadWriteCloser) {
	defer metrics.MeasureSince([]string{"client", "monitor", "monitor"}, time.Now())
	// defer conn.Close()

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

	// var buf bytes.Buffer

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	streamWriter := newStreamWriter(512)

	streamLog := log.New(&log.LoggerOptions{
		Level:  logLevel,
		Output: streamWriter,
	})
	m.c.logger.RegisterSink(streamLog)
	defer m.c.logger.DeregisterSink(streamLog)

	go func() {
		for {
			if _, err := conn.Read(nil); err != nil {
				// One end of the pipe was explicitly closed, exit cleanly
				cancel()
				return
			}
			select {
			case <-ctx.Done():
				return
			}
		}
	}()

	var streamErr error
OUTER:
	for {
		select {
		case log := <-streamWriter.logCh:
			var resp cstructs.StreamErrWrapper
			if err := encoder.Encode(resp); err != nil {
				streamErr = err
				break OUTER
			}
			resp.Payload = []byte(log)
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

type streamWriter struct {
	sync.Mutex
	logs         []string
	logCh        chan string
	index        int
	droppedCount int
}

func newStreamWriter(buf int) *streamWriter {
	return &streamWriter{
		logs:  make([]string, buf),
		logCh: make(chan string, buf),
		index: 0,
	}
}

func (d *streamWriter) Write(p []byte) (n int, err error) {
	d.Lock()
	defer d.Unlock()

	// Strip off newlines at the end if there are any since we store
	// individual log lines in the agent.
	n = len(p)
	if p[n-1] == '\n' {
		p = p[:n-1]
	}

	d.logs[d.index] = string(p)
	d.index = (d.index + 1) % len(d.logs)

	select {
	case d.logCh <- string(p):
	default:
		d.droppedCount++
	}
	return
}
