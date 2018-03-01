package state

import (
	"testing"
	"time"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

func TestStateStore_AddSingleNodeEvent(t *testing.T) {
	require := require.New(t)
	state := testStateStore(t)

	node := mock.Node()

	err := state.UpsertNode(1000, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	nodeEvent := &structs.NodeEvent{
		Message:   "failed",
		Subsystem: "Driver",
		Timestamp: time.Now().Unix(),
	}
	err = state.AddNodeEvent(1001, node.ID, nodeEvent)
	require.Nil(err)

	ws := memdb.NewWatchSet()
	actualNode, err := state.NodeByID(ws, node.ID)
	require.Nil(err)
	require.Equal(1, len(actualNode.NodeEvents))
	require.Equal(nodeEvent, actualNode.NodeEvents[0])
}

// To prevent stale node events from accumulating, we limit the number of
// stored node events to 10.
func TestStateStore_NodeEvents_RetentionWindow(t *testing.T) {
	require := require.New(t)
	state := testStateStore(t)

	node := mock.Node()

	err := state.UpsertNode(1000, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	for i := 1; i <= 20; i++ {
		nodeEvent := &structs.NodeEvent{
			Message:   "failed",
			Subsystem: "Driver",
			Timestamp: time.Now().Unix(),
		}
		err := state.AddNodeEvent(uint64(i), node.ID, nodeEvent)
		require.Nil(err)
	}

	ws := memdb.NewWatchSet()
	actualNode, err := state.NodeByID(ws, node.ID)
	require.Nil(err)

	require.Equal(10, len(actualNode.NodeEvents))
	require.Equal(uint64(11), actualNode.NodeEvents[0].CreateIndex)
	require.Equal(uint64(20), actualNode.NodeEvents[len(actualNode.NodeEvents)-1].CreateIndex)
}
