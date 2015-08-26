package fingerprint

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

func TestOSFingerprint(t *testing.T) {
	f := NewOSFingerprint(testLogger())
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
	if node.Attributes["os"] == "" {
		t.Fatalf("missing OS")
	}
	if node.Attributes["cpu.numcores"] == "" {
		t.Fatalf("Missing Num Cores")
	}
	if node.Attributes["cpu.modelname"] == "" {
		t.Fatalf("Missing Model Name")
	}
}
