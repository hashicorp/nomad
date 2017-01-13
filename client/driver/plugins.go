package driver

import (
	"io"
	"log"
	"net"
	"strings"

	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/logutils"
)

var HandshakeConfig = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "NOMAD_PLUGIN_MAGIC_COOKIE",
	MagicCookieValue: "e4327c2e01eabfd75a8a67adb114fb34a757d57eee7728d857a8cec6e91a7255",
}

func GetPluginMap(w io.Writer, logLevel string) map[string]plugin.Plugin {
	e := new(ExecutorPlugin)
	filter := &logutils.LevelFilter{
		Levels:   []logutils.LogLevel{"TRACE", "DEBUG", "INFO", "WARN", "ERR"},
		MinLevel: logutils.LogLevel(strings.ToUpper(logLevel)),
		Writer:   w,
	}

	e.logger = log.New(filter, "", log.LstdFlags|log.Lmicroseconds)

	return map[string]plugin.Plugin{
		"executor": e,
	}
}

// ExecutorReattachConfig is the config that we seralize and de-serialize and
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
