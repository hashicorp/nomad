package driver

import (
	"log"
	"os"
	"testing"
)

func testLogger() *log.Logger {
	return log.New(os.Stderr, "", log.LstdFlags)
}

func TestExecDriver_Fingerprint(t *testing.T) {
	d := NewExecDriver(testLogger())
	apply, err := d.Fingerprint(nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !apply {
		t.Fatalf("should apply")
	}
}
