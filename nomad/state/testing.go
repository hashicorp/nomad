package state

import (
	testing "github.com/mitchellh/go-testing-interface"

	"github.com/hashicorp/nomad/helper/testlog"
)

func TestStateStore(t testing.T) *StateStore {
	config := &StateStoreConfig{
		Logger: testlog.HCLogger(t),
		Region: "global",
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
