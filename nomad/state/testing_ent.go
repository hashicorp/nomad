// +build ent pro

package state

import (
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/mitchellh/go-testing-interface"
)

func TestInitState(t testing.T, state *StateStore) {
	testInitDefaultNamespace(t, state)
}

func testInitDefaultNamespace(t testing.T, state *StateStore) {
	d := mock.Namespace()
	d.Name = structs.DefaultNamespace
	if err := state.UpsertNamespaces(1, []*structs.Namespace{d}); err != nil {
		t.Fatalf("failed to upsert default namespace: %v", err)
	}
}
