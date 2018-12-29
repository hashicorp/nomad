package state

import (
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/mitchellh/go-testing-interface"
)

func TestStateStore(t testing.T) *StateStore {
	config := &StateStoreConfig{
		LogOutput: testlog.NewWriter(t),
		Region:    "global",
	}
	state, err := NewStateStore(config)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if state == nil {
		t.Fatalf("missing state")
	}
	TestInitState(t, state)
	return state
}
