package executor

import (
	"io"
	"net"

	hclog "github.com/hashicorp/go-hclog"
	plugin "github.com/hashicorp/go-plugin"
)

// ExecutorConfig is the config that Nomad passes to the executor
type ExecutorConfig struct {

	// LogFile is the file to which Executor logs
	LogFile string

	// LogLevel is the level of the logs to putout
	LogLevel string

	// FSIsolation if set will use an executor implementation that support
	// filesystem isolation
	FSIsolation bool
}

func GetPluginMap(w io.Writer, logLevel hclog.Level, fsIsolation bool) map[string]plugin.Plugin {
	e := new(ExecutorPlugin)

	e.logger = hclog.New(&hclog.LoggerOptions{
		Output: w,
		Level:  logLevel,
	})

	e.fsIsolation = fsIsolation

	return map[string]plugin.Plugin{
		"executor": e,
	}
}

// ExecutorReattachConfig is the config that we serialize and de-serialize and
// store in disk
type PluginReattachConfig struct {
	Pid      int
	AddrNet  string
	AddrName string
}

// PluginConfig returns a config from an ExecutorReattachConfig
func (c *PluginReattachConfig) PluginConfig() *plugin.ReattachConfig {
	var addr net.Addr
	switch c.AddrNet {
	case "unix", "unixgram", "unixpacket":
		addr, _ = net.ResolveUnixAddr(c.AddrNet, c.AddrName)
	case "tcp", "tcp4", "tcp6":
		addr, _ = net.ResolveTCPAddr(c.AddrNet, c.AddrName)
	}
	return &plugin.ReattachConfig{Pid: c.Pid, Addr: addr}
}

func NewPluginReattachConfig(c *plugin.ReattachConfig) *PluginReattachConfig {
	return &PluginReattachConfig{Pid: c.Pid, AddrNet: c.Addr.Network(), AddrName: c.Addr.String()}
}
