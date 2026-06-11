package testlog

import (
	"fmt"
	"io"
	"os"

	"github.com/hashicorp/go-hclog"
)

// HCLogger returns a new test hc-logger.
//
// Default log level is TRACE. Set NOMAD_TEST_LOG_LEVEL for custom log level.
func HCLogger(t LogPrinter) hclog.InterceptLogger {
	logger, _ := HCLoggerNode(t, -1)
	return logger
}

// HCLoggerTestLevel returns the level in which hc log should emit logs.
//
// Default log level is TRACE. Set NOMAD_TEST_LOG_LEVEL for custom log level.
func HCLoggerTestLevel() hclog.Level {
	level := hclog.Trace
	envLogLevel := os.Getenv("NOMAD_TEST_LOG_LEVEL")
	if envLogLevel != "" {
		level = hclog.LevelFromString(envLogLevel)
	}
	return level
}

// HCLoggerNode returns a new hc-logger, but with a prefix indicating the node number
// on each log line. Useful for TestServer in tests with more than one server.
//
// Default log level is TRACE. Set NOMAD_TEST_LOG_LEVEL for custom log level.
func HCLoggerNode(t LogPrinter, node int32) (hclog.InterceptLogger, io.Writer) {
	var output io.Writer = os.Stderr
	if node > -1 {
		output = NewPrefixWriter(t, fmt.Sprintf("node-%03d ", node))
	}
	opts := &hclog.LoggerOptions{
		Level:           HCLoggerTestLevel(),
		Output:          output,
		IncludeLocation: true,
	}
	return hclog.NewInterceptLogger(opts), output
}
