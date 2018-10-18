package driver

import (
	"io"
	"net"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
)

var HandshakeConfig = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "NOMAD_PLUGIN_MAGIC_COOKIE",
	MagicCookieValue: "e4327c2e01eabfd75a8a67adb114fb34a757d57eee7728d857a8cec6e91a7255",
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
