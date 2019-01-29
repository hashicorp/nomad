package command

import (
	"encoding/json"
	"io"
	"os"
	"strings"

	hclog "github.com/hashicorp/go-hclog"
	log "github.com/hashicorp/go-hclog"
	plugin "github.com/hashicorp/go-plugin"

	"github.com/hashicorp/nomad/drivers/shared/executor"
	"github.com/hashicorp/nomad/lib/circbufwriter"
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
	bufferedStderrW := circbufwriter.New(os.Stderr, circleBufferSize)

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
