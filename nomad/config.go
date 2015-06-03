package nomad

import (
	"fmt"
	"io"
	"net"

	"github.com/hashicorp/raft"
)

// These are the protocol versions that Nomad can understand
const (
	ProtocolVersionMin uint8 = 1
	ProtocolVersionMax       = 1
)

var (
	DefaultRPCAddr = &net.TCPAddr{IP: net.ParseIP("0.0.0.0"), Port: 4646}
)

// Config is used to parameterize the server
type Config struct {
	// Bootstrap mode is used to bring up the first Consul server.
	// It is required so that it can elect a leader without any
	// other nodes being present
	Bootstrap bool

	// DataDir is the directory to store our state in
	DataDir string

	// DevMode is used for development purposes only and limits the
	// use of persistence or state.
	DevMode bool

	// LogOutput is the location to write logs to. If this is not set,
	// logs will go to stderr.
	LogOutput io.Writer

	// ProtocolVersion is the protocol version to speak. This must be between
	// ProtocolVersionMin and ProtocolVersionMax.
	ProtocolVersion uint8

	// RPCAddr is the RPC address used by Nomad. This should be reachable
	// by the other servers and clients
	RPCAddr *net.TCPAddr

	// RPCAdvertise is the address that is advertised to other nodes for
	// the RPC endpoint. This can differ from the RPC address, if for example
	// the RPCAddr is unspecified "0.0.0.0:4646", but this address must be
	// reachable
	RPCAdvertise *net.TCPAddr

	// RaftConfig is the configuration used for Raft in the local DC
	RaftConfig *raft.Config

	// RequireTLS ensures that all RPC traffic is protected with TLS
	RequireTLS bool
}

// CheckVersion is used to check if the ProtocolVersion is valid
func (c *Config) CheckVersion() error {
	if c.ProtocolVersion < ProtocolVersionMin {
		return fmt.Errorf("Protocol version '%d' too low. Must be in range: [%d, %d]",
			c.ProtocolVersion, ProtocolVersionMin, ProtocolVersionMax)
	} else if c.ProtocolVersion > ProtocolVersionMax {
		return fmt.Errorf("Protocol version '%d' too high. Must be in range: [%d, %d]",
			c.ProtocolVersion, ProtocolVersionMin, ProtocolVersionMax)
	}
	return nil
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	c := &Config{
		ProtocolVersion: ProtocolVersionMax,
		RaftConfig:      raft.DefaultConfig(),
		RPCAddr:         DefaultRPCAddr,
	}
	return c
}
