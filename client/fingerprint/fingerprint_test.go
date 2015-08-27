package fingerprint

// This file contains helper methods for testing fingerprinters

import (
	"log"
	"os"
	"testing"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
)

func testLogger() *log.Logger {
	return log.New(os.Stderr, "", log.LstdFlags)
}

func assertFingerprintOK(t *testing.T, fp Fingerprint, node *structs.Node) {
	ok, err := fp.Fingerprint(new(config.Config), node)
	if err != nil {
		t.Fatalf("Failed to fingerprint: %s", err)
	}
	if !ok {
		t.Fatalf("Failed to apply node attributes")
	}
}

func assertNodeAttributeContains(t *testing.T, node *structs.Node, attribute string) {
	actual, found := node.Attributes[attribute]
	if !found {
		t.Errorf("Expected to find `%s`\n\n[DEBUG] %#v", attribute, node)
		return
	}
	if actual == "" {
		t.Errorf("Expected non-empty value for `%s`\n\n[DEBUG] %#v", attribute, node)
	}
}

func assertNodeAttributeEquals(t *testing.T, node *structs.Node, attribute string, expected string) {
	actual, found := node.Attributes[attribute]
	if !found {
		t.Errorf("Expected to find `%s`; unable to check value\n\n[DEBUG] %#v", attribute, node)
		return
	}
	if expected != actual {
		t.Errorf("Expected `%s` to be `%s`, found `%s`\n\n[DEBUG] %#v", attribute, expected, actual, node)
	}
}
