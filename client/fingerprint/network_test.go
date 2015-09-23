package fingerprint

import (
	"net"
	"testing"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
)

func TestNetworkFingerprint_basic(t *testing.T) {
	f := NewNetworkFingerprinter(testLogger())
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

	assertNodeAttributeContains(t, node, "network.ip-address")

	ip := node.Attributes["network.ip-address"]
	match := net.ParseIP(ip)
	if match == nil {
		t.Fatalf("Bad IP match: %s", ip)
	}

	if node.Resources == nil || len(node.Resources.Networks) == 0 {
		t.Fatal("Expected to find Network Resources")
	}

	// Test at least the first Network Resource
	net := node.Resources.Networks[0]
	if net.IP == "" {
		t.Fatal("Expected Network Resource to not be empty")
	}
	if net.CIDR == "" {
		t.Fatal("Expected Network Resource to have a CIDR")
	}
	if net.Device == "" {
		t.Fatal("Expected Network Resource to have a Device Name")
	}
}
