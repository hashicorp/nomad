package fingerprint

// This file contains helper methods for testing fingerprinters

import (
	"log"
	"os"
	"testing"

	"github.com/hashicorp/nomad/client/config"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/structs"
)

func testLogger() *log.Logger {
	return log.New(os.Stderr, "", log.LstdFlags)
}

func assertFingerprintOK(t *testing.T, fp Fingerprint, node *structs.Node) *cstructs.FingerprintResponse {
	request := &cstructs.FingerprintRequest{Config: new(config.Config), Node: node}
	var response cstructs.FingerprintResponse
	err := fp.Fingerprint(request, &response)
	if err != nil {
		t.Fatalf("Failed to fingerprint: %s", err)
	}

	if len(response.Attributes) == 0 {
		t.Fatalf("Failed to apply node attributes")
	}

	return &response
}

func assertNodeAttributeContains(t *testing.T, nodeAttributes map[string]string, attribute string) {
	if nodeAttributes == nil {
		t.Errorf("expected an initialized map for node attributes")
		return
	}

	actual, found := nodeAttributes[attribute]
	if !found {
		t.Errorf("Expected to find Attribute `%s`\n\n[DEBUG] %#v", attribute, nodeAttributes)
		return
	}
	if actual == "" {
		t.Errorf("Expected non-empty Attribute value for `%s`\n\n[DEBUG] %#v", attribute, nodeAttributes)
	}
}

func assertNodeAttributeEquals(t *testing.T, nodeAttributes map[string]string, attribute string, expected string) {
	if nodeAttributes == nil {
		t.Errorf("expected an initialized map for node attributes")
		return
	}
	actual, found := nodeAttributes[attribute]
	if !found {
		t.Errorf("Expected to find Attribute `%s`; unable to check value\n\n[DEBUG] %#v", attribute, nodeAttributes)
		return
	}
	if expected != actual {
		t.Errorf("Expected `%s` Attribute to be `%s`, found `%s`\n\n[DEBUG] %#v", attribute, expected, actual, nodeAttributes)
	}
}

func assertNodeLinksContains(t *testing.T, nodeLinks map[string]string, link string) {
	if nodeLinks == nil {
		t.Errorf("expected an initialized map for node links")
		return
	}
	actual, found := nodeLinks[link]
	if !found {
		t.Errorf("Expected to find Link `%s`\n\n[DEBUG]", link)
		return
	}
	if actual == "" {
		t.Errorf("Expected non-empty Link value for `%s`\n\n[DEBUG]", link)
	}
}
