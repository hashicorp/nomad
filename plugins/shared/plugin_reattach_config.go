package shared

import (
	"fmt"
	"net"

	plugin "github.com/hashicorp/go-plugin"
)

// ReattachConfig is a wrapper around plugin.ReattachConfig to better support
// serialization
type ReattachConfig struct {
	Protocol string
	Network  string
	Addr     string
	Pid      int
}

// ReattachConfigToGoPlugin converts a ReattachConfig wrapper struct into a go
// plugin ReattachConfig struct
func ReattachConfigToGoPlugin(rc *ReattachConfig) (*plugin.ReattachConfig, error) {
	if rc == nil {
		return nil, fmt.Errorf("nil ReattachConfig cannot be converted")
	}

	plug := &plugin.ReattachConfig{
		Protocol: plugin.Protocol(rc.Protocol),
		Pid:      rc.Pid,
	}

	switch rc.Network {
	case "tcp", "tcp4", "tcp6":
		addr, err := net.ResolveTCPAddr(rc.Network, rc.Addr)
		if err != nil {
			return nil, err
		}
		plug.Addr = addr
	case "udp", "udp4", "udp6":
		addr, err := net.ResolveUDPAddr(rc.Network, rc.Addr)
		if err != nil {
			return nil, err
		}
		plug.Addr = addr
	case "unix", "unixgram", "unixpacket":
		addr, err := net.ResolveUnixAddr(rc.Network, rc.Addr)
		if err != nil {
			return nil, err
		}
		plug.Addr = addr
	default:
		return nil, fmt.Errorf("unknown network: %s", rc.Network)
	}

	return plug, nil
}

// ReattachConfigFromGoPlugin converts a go plugin ReattachConfig into a
// ReattachConfig wrapper struct
func ReattachConfigFromGoPlugin(plug *plugin.ReattachConfig) *ReattachConfig {
	if plug == nil {
		return nil
	}

	rc := &ReattachConfig{
		Protocol: string(plug.Protocol),
		Network:  plug.Addr.Network(),
		Addr:     plug.Addr.String(),
		Pid:      plug.Pid,
	}

	return rc
}
