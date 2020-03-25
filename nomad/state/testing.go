package state

import (
	testing "github.com/mitchellh/go-testing-interface"

	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
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

// CreateTestPlugin is a helper that generates the node + fingerprint results necessary to
// create a CSIPlugin by directly inserting into the state store. It's exported for use in
// other test packages
func CreateTestCSIPlugin(s *StateStore, id string) func() {
	// Create some nodes
	ns := make([]*structs.Node, 3)
	for i := range ns {
		n := mock.Node()
		ns[i] = n
	}

	// Install healthy plugin fingerprinting results
	ns[0].CSIControllerPlugins = map[string]*structs.CSIInfo{
		id: {
			PluginID:                 id,
			AllocID:                  uuid.Generate(),
			Healthy:                  true,
			HealthDescription:        "healthy",
			RequiresControllerPlugin: true,
			RequiresTopologies:       false,
			ControllerInfo: &structs.CSIControllerInfo{
				SupportsReadOnlyAttach:           true,
				SupportsAttachDetach:             true,
				SupportsListVolumes:              true,
				SupportsListVolumesAttachedNodes: false,
			},
		},
	}

	// Install healthy plugin fingerprinting results
	for _, n := range ns[1:] {
		n.CSINodePlugins = map[string]*structs.CSIInfo{
			id: {
				PluginID:                 id,
				AllocID:                  uuid.Generate(),
				Healthy:                  true,
				HealthDescription:        "healthy",
				RequiresControllerPlugin: true,
				RequiresTopologies:       false,
				NodeInfo: &structs.CSINodeInfo{
					ID:                      n.ID,
					MaxVolumes:              64,
					RequiresNodeStageVolume: true,
				},
			},
		}
	}

	// Insert them into the state store
	index := uint64(999)
	for _, n := range ns {
		index++
		s.UpsertNode(index, n)
	}

	ids := make([]string, len(ns))
	for i, n := range ns {
		ids[i] = n.ID
	}

	// Return cleanup function that deletes the nodes
	return func() {
		index++
		s.DeleteNode(index, ids)
	}
}
