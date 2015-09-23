package fingerprint

import (
	"net"
	"runtime"
	"testing"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
)

func TestNetworkFingerprint_basic(t *testing.T) {
	f := NewUnixNetworkFingerprinter(testLogger())
	node := &structs.Node{
		Attributes: make(map[string]string),
	}

	ok, err := f.Fingerprint(&config.Config{}, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !ok {
		t.Fatalf("should apply")
	}

	// Darwin uses en0 for the default device, and does not have a standard
	// location for the linkspeed file, so we skip these
	if "darwin" != runtime.GOOS {
		assertNodeAttributeContains(t, node, "network.throughput")
	}
	assertNodeAttributeContains(t, node, "network.ip-address")

	ip := node.Attributes["network.ip-address"]
	match := net.ParseIP(ip)
	if match == nil {
		t.Fatalf("Bad IP match: %s", ip)
	}
}
