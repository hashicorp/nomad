package state

import (
	"os"

	"github.com/mitchellh/go-testing-interface"
)

func TestStateStore(t testing.T) *StateStore {
	config := &StateStoreConfig{
		LogOutput: os.Stderr,
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
