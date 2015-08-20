package driver

import (
	"log"
	"os"
	"testing"

	"github.com/hashicorp/nomad/nomad/structs"
)

func testLogger() *log.Logger {
	return log.New(os.Stderr, "", log.LstdFlags)
}

func TestExecDriver_Fingerprint(t *testing.T) {
	d := NewExecDriver(testLogger())
	node := &structs.Node{
		Attributes: make(map[string]string),
	}
	apply, err := d.Fingerprint(node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !apply {
		t.Fatalf("should apply")
	}
	if node.Attributes["driver.exec"] == "" {
		t.Fatalf("missing driver")
	}
}
