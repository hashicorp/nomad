package volumewatcher

import (
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

// Create a client node with plugin info
func testNode(node *structs.Node, plugin *structs.CSIPlugin, s *state.StateStore) *structs.Node {
	if node != nil {
		return node
	}
	node = mock.Node()
	node.Attributes["nomad.version"] = "0.11.0" // client RPCs not supported on early version
	node.CSINodePlugins = map[string]*structs.CSIInfo{
		plugin.ID: {
			PluginID:                 plugin.ID,
			Healthy:                  true,
			RequiresControllerPlugin: plugin.ControllerRequired,
			NodeInfo:                 &structs.CSINodeInfo{},
		},
	}
	if plugin.ControllerRequired {
		node.CSIControllerPlugins = map[string]*structs.CSIInfo{
			plugin.ID: {
				PluginID:                 plugin.ID,
				Healthy:                  true,
				RequiresControllerPlugin: true,
				ControllerInfo: &structs.CSIControllerInfo{
					SupportsReadOnlyAttach:           true,
					SupportsAttachDetach:             true,
					SupportsListVolumes:              true,
					SupportsListVolumesAttachedNodes: false,
				},
			},
		}
	} else {
		node.CSIControllerPlugins = map[string]*structs.CSIInfo{}
	}
	s.UpsertNode(99, node)
	return node
}

// Create a test volume with claim info
func testVolume(vol *structs.CSIVolume, plugin *structs.CSIPlugin, alloc *structs.Allocation, nodeID string) *structs.CSIVolume {
	if vol != nil {
		return vol
	}
	vol = mock.CSIVolume(plugin)
	vol.ControllerRequired = plugin.ControllerRequired

	vol.ReadAllocs = map[string]*structs.Allocation{alloc.ID: alloc}
	vol.ReadClaims = map[string]*structs.CSIVolumeClaim{
		alloc.ID: {
			AllocationID: alloc.ID,
			NodeID:       nodeID,
			Mode:         structs.CSIVolumeClaimRead,
			State:        structs.CSIVolumeClaimStateTaken,
		},
	}
	return vol
}

// COMPAT(1.0): the claim fields were added after 0.11.1; this
// mock and the associated test cases can be removed for 1.0
func testOldVolume(vol *structs.CSIVolume, plugin *structs.CSIPlugin, alloc *structs.Allocation, nodeID string) *structs.CSIVolume {
	if vol != nil {
		return vol
	}
	vol = mock.CSIVolume(plugin)
	vol.ControllerRequired = plugin.ControllerRequired

	vol.ReadAllocs = map[string]*structs.Allocation{alloc.ID: alloc}
	return vol
}

type MockRPCServer struct {
	state *state.StateStore

	// mock responses for ClientCSI.NodeDetachVolume
	nextCSINodeDetachResponse *cstructs.ClientCSINodeDetachVolumeResponse
	nextCSINodeDetachError    error
	countCSINodeDetachVolume  int

	// mock responses for ClientCSI.ControllerDetachVolume
	nextCSIControllerDetachVolumeResponse *cstructs.ClientCSIControllerDetachVolumeResponse
	nextCSIControllerDetachError          error
	countCSIControllerDetachVolume        int

	countUpdateClaims       int
	countUpsertVolumeClaims int
}

func (srv *MockRPCServer) ControllerDetachVolume(args *cstructs.ClientCSIControllerDetachVolumeRequest, reply *cstructs.ClientCSIControllerDetachVolumeResponse) error {
	reply = srv.nextCSIControllerDetachVolumeResponse
	srv.countCSIControllerDetachVolume++
	return srv.nextCSIControllerDetachError
}

func (srv *MockRPCServer) NodeDetachVolume(args *cstructs.ClientCSINodeDetachVolumeRequest, reply *cstructs.ClientCSINodeDetachVolumeResponse) error {
	reply = srv.nextCSINodeDetachResponse
	srv.countCSINodeDetachVolume++
	return srv.nextCSINodeDetachError

}

func (srv *MockRPCServer) UpsertVolumeClaims(*structs.CSIVolumeClaimBatchRequest) (uint64, error) {
	srv.countUpsertVolumeClaims++
	return 0, nil
}

func (srv *MockRPCServer) State() *state.StateStore { return srv.state }

func (srv *MockRPCServer) UpdateClaims(claims []structs.CSIVolumeClaimRequest) (uint64, error) {
	srv.countUpdateClaims++
	return 0, nil
}

type MockBatchingRPCServer struct {
	MockRPCServer
	volumeUpdateBatcher *VolumeUpdateBatcher
}

func (srv *MockBatchingRPCServer) UpdateClaims(claims []structs.CSIVolumeClaimRequest) (uint64, error) {
	srv.countUpdateClaims++
	return srv.volumeUpdateBatcher.CreateUpdate(claims).Results()
}

type MockStatefulRPCServer struct {
	MockRPCServer
	volumeUpdateBatcher *VolumeUpdateBatcher
}

func (srv *MockStatefulRPCServer) UpsertVolumeClaims(batch *structs.CSIVolumeClaimBatchRequest) (uint64, error) {
	srv.countUpsertVolumeClaims++
	index, _ := srv.state.LatestIndex()
	for _, req := range batch.Claims {
		index++
		err := srv.state.CSIVolumeClaim(index, req.RequestNamespace(),
			req.VolumeID, req.ToClaim())
		if err != nil {
			return 0, err
		}
	}
	return index, nil
}
