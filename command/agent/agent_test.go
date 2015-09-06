package agent

import (
	"io/ioutil"
	"os"
	"sync/atomic"
	"testing"
)

var nextPort uint32 = 17000

func getPort() int {
	return int(atomic.AddUint32(&nextPort, 1))
}

func tmpDir(t *testing.T) string {
	dir, err := ioutil.TempDir("", "nomad")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	return dir
}

func makeAgent(t *testing.T, cb func(*Config)) (string, *Agent) {
	dir := tmpDir(t)
	conf := DevConfig()

	if cb != nil {
		cb(conf)
	}

	agent, err := NewAgent(conf, os.Stderr)
	if err != nil {
		os.RemoveAll(dir)
		t.Fatalf("err: %v", err)
	}
	return dir, agent
}
