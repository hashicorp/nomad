package fingerprint

import (
	"log"
	"os"
	"testing"

	"github.com/hashicorp/nomad/nomad/structs"
)

func testLogger() *log.Logger {
	return log.New(os.Stderr, "", log.LstdFlags)
}

func TestOSFingerprint(t *testing.T) {
	f := NewOSFingerprint(testLogger())
	node := &structs.Node{
		Attributes: make(map[string]string),
	}
	ok, err := f.Fingerprint(node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !ok {
		t.Fatalf("should apply")
	}
	if node.Attributes["os"] == "" {
		t.Fatalf("missing OS")
	}
}
