package sdk

import (
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/nomad/sdk/portfree"
)

var (
	lock sync.Mutex
	id   int
)

func nextID() int {
	lock.Lock()
	defer lock.Unlock()
	id = id + 1
	return id
}

type Stop func()

type Wait func()

type PortsConfig struct {
	RPC     int `json:"server,omitempty"` // Consul (-server-port, ports.server) (8300)
	DNS     int `json:"dns,omitempty"`    // Consul
	HTTP    int `json:"http,omitempty"`
	HTTPS   int `json:"https,omitempty"`
	GRPC    int `json:"grpc,omitempty"`
	SerfLAN int `json:"serf_lan,omitempty"`
	SerfWAN int `json:"serf_wan,omitempty"`
}

func (pc *PortsConfig) Cleanup() {
	// the kernel will cleanup the ports
}

func FreeConsulPorts(t *testing.T) *PortsConfig {
	ports := portfree.New(t).Get(7)
	return &PortsConfig{
		RPC:     ports[0],
		DNS:     ports[1],
		HTTP:    ports[2],
		HTTPS:   ports[3],
		GRPC:    ports[4],
		SerfLAN: ports[5],
		SerfWAN: ports[6],
	}
}

func logOutput(level string, writer io.Writer) io.Writer {
	if strings.ToLower(level) == "off" {
		return nil
	}
	return writer
}
