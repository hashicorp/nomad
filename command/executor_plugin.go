package command

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/armon/circbuf"
	hclog "github.com/hashicorp/go-hclog"
	log "github.com/hashicorp/go-hclog"
	plugin "github.com/hashicorp/go-plugin"

	"github.com/hashicorp/nomad/drivers/shared/executor"
	"github.com/hashicorp/nomad/plugins/base"
)

const (
	// circleBufferSize is the size of the in memory ring buffer used for
	// go-plugin logging to stderr. When the buffer exceeds this size before
	// flushing it will begin overwriting data
	circleBufferSize = 64 * 1024
)

type ExecutorPluginCommand struct {
	Meta
}

func (e *ExecutorPluginCommand) Help() string {
	helpText := `
	This is a command used by Nomad internally to launch an executor plugin"
	`
	return strings.TrimSpace(helpText)
}

func (e *ExecutorPluginCommand) Synopsis() string {
	return "internal - launch an executor plugin"
}

func (e *ExecutorPluginCommand) Run(args []string) int {
	if len(args) != 1 {
		e.Ui.Error("json configuration not provided")
		return 1
	}

	config := args[0]
	var executorConfig executor.ExecutorConfig
	if err := json.Unmarshal([]byte(config), &executorConfig); err != nil {
		return 1
	}

	f, err := os.OpenFile(executorConfig.LogFile, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
	if err != nil {
		e.Ui.Error(err.Error())
		return 1
	}

	// If the client detatches from go-plugin it will block on logging to stderr.
	// This buffered writer will never block on write, and instead buffer the
	// writes to a ring buffer.
	bufferedStderrW := newCircbufWriter(os.Stderr)

	// Tee the logs to stderr and the file so that they are streamed to the
	// client
	out := io.MultiWriter(f, bufferedStderrW)

	// Create the logger
	logger := log.New(&log.LoggerOptions{
		Level:      hclog.LevelFromString(executorConfig.LogLevel),
		JSONFormat: true,
		Output:     out,
	})

	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: base.Handshake,
		Plugins: executor.GetPluginMap(
			logger,
			executorConfig.FSIsolation,
		),
		GRPCServer: plugin.DefaultGRPCServer,
		Logger:     logger,
	})
	return 0
}

type circbufWriter struct {
	buf     *circbuf.Buffer
	err     error
	bufLock sync.Mutex
	wr      io.Writer
	flushCh chan struct{}
}

func newCircbufWriter(w io.Writer) *circbufWriter {
	buf, _ := circbuf.NewBuffer(circleBufferSize)
	c := &circbufWriter{
		buf:     buf,
		wr:      w,
		flushCh: make(chan struct{}),
	}
	go c.flushLoop()
	return c
}

func (c *circbufWriter) Write(p []byte) (nn int, err error) {
	if c.err != nil {
		return nn, c.err
	}
	c.bufLock.Lock()
	nn, err = c.buf.Write(p)
	c.bufLock.Unlock()

	select {
	case c.flushCh <- struct{}{}:
	default:
	}
	return nn, err
}

func (c *circbufWriter) Close() error {
	var err error
	if wc, ok := c.wr.(io.WriteCloser); ok {
		err = wc.Close()
	}

	close(c.flushCh)
	return err
}

func (c *circbufWriter) flushLoop() {
	timer := time.NewTimer(time.Millisecond * 100)
	for {
		select {
		case _, ok := <-c.flushCh:
			if !ok {
				return
			}
			c.err = c.flush()
		case <-timer.C:
			c.err = c.flush()
		}
	}
}

func (c *circbufWriter) flush() error {
	c.bufLock.Lock()
	b := c.buf.Bytes()
	c.buf.Reset()
	c.bufLock.Unlock()

	var err error
	if len(b) > 0 {
		_, err = c.wr.Write(b)
	}
	return err
}
