package drainer

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

func testNodeDrainWatcher(t *testing.T) (*nodeDrainWatcher, *state.StateStore, *MockNodeTracker) {
	t.Helper()
	state := state.TestStateStore(t)
	limiter := rate.NewLimiter(100.0, 100)
	logger := testlog.HCLogger(t)
	m := NewMockNodeTracker()
	w := NewNodeDrainWatcher(context.Background(), limiter, state, logger, m)
	return w, state, m
}

func TestNodeDrainWatcher_Interface(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	w, _, _ := testNodeDrainWatcher(t)
	require.Implements((*DrainingNodeWatcher)(nil), w)
}

func TestNodeDrainWatcher_AddDraining(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	_, state, m := testNodeDrainWatcher(t)

	// Create two nodes, one draining and one not draining
	n1, n2 := mock.Node(), mock.Node()
	n2.DrainStrategy = &structs.DrainStrategy{
		DrainSpec: structs.DrainSpec{
			Deadline: time.Hour,
		},
		ForceDeadline: time.Now().Add(time.Hour),
	}

	require.Nil(state.UpsertNode(100, n1))
	require.Nil(state.UpsertNode(101, n2))

	testutil.WaitForResult(func() (bool, error) {
		return len(m.Events) == 1, nil
	}, func(err error) {
		t.Fatal("No node drain events")
	})

	tracked := m.TrackedNodes()
	require.NotContains(tracked, n1.ID)
	require.Contains(tracked, n2.ID)
	require.Equal(n2, tracked[n2.ID])

}

func TestNodeDrainWatcher_Remove(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	_, state, m := testNodeDrainWatcher(t)

	// Create a draining node
	n := mock.Node()
	n.DrainStrategy = &structs.DrainStrategy{
		DrainSpec: structs.DrainSpec{
			Deadline: time.Hour,
		},
		ForceDeadline: time.Now().Add(time.Hour),
	}

	// Wait for it to be tracked
	require.Nil(state.UpsertNode(100, n))
	testutil.WaitForResult(func() (bool, error) {
		return len(m.Events) == 1, nil
	}, func(err error) {
		t.Fatal("No node drain events")
	})

	tracked := m.TrackedNodes()
	require.Contains(tracked, n.ID)
	require.Equal(n, tracked[n.ID])

	// Change the node to be not draining and wait for it to be untracked
	require.Nil(state.UpdateNodeDrain(101, n.ID, nil, false, nil))
	testutil.WaitForResult(func() (bool, error) {
		return len(m.Events) == 2, nil
	}, func(err error) {
		t.Fatal("No new node drain events")
	})

	tracked = m.TrackedNodes()
	require.NotContains(tracked, n.ID)
}

func TestNodeDrainWatcher_Remove_Nonexistent(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	_, state, m := testNodeDrainWatcher(t)

	// Create a draining node
	n := mock.Node()
	n.DrainStrategy = &structs.DrainStrategy{
		DrainSpec: structs.DrainSpec{
			Deadline: time.Hour,
		},
		ForceDeadline: time.Now().Add(time.Hour),
	}

	// Wait for it to be tracked
	require.Nil(state.UpsertNode(100, n))
	testutil.WaitForResult(func() (bool, error) {
		return len(m.Events) == 1, nil
	}, func(err error) {
		t.Fatal("No node drain events")
	})

	tracked := m.TrackedNodes()
	require.Contains(tracked, n.ID)
	require.Equal(n, tracked[n.ID])

	// Delete the node
	require.Nil(state.DeleteNode(101, n.ID))
	testutil.WaitForResult(func() (bool, error) {
		return len(m.Events) == 2, nil
	}, func(err error) {
		t.Fatal("No new node drain events")
	})

	tracked = m.TrackedNodes()
	require.NotContains(tracked, n.ID)
}

func TestNodeDrainWatcher_Update(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	_, state, m := testNodeDrainWatcher(t)

	// Create a draining node
	n := mock.Node()
	n.DrainStrategy = &structs.DrainStrategy{
		DrainSpec: structs.DrainSpec{
			Deadline: time.Hour,
		},
		ForceDeadline: time.Now().Add(time.Hour),
	}

	// Wait for it to be tracked
	require.Nil(state.UpsertNode(100, n))
	testutil.WaitForResult(func() (bool, error) {
		return len(m.Events) == 1, nil
	}, func(err error) {
		t.Fatal("No node drain events")
	})

	tracked := m.TrackedNodes()
	require.Contains(tracked, n.ID)
	require.Equal(n, tracked[n.ID])

	// Change the node to have a new spec
	s2 := n.DrainStrategy.Copy()
	s2.Deadline += time.Hour
	require.Nil(state.UpdateNodeDrain(101, n.ID, s2, false, nil))

	// Wait for it to be updated
	testutil.WaitForResult(func() (bool, error) {
		return len(m.Events) == 2, nil
	}, func(err error) {
		t.Fatal("No new node drain events")
	})

	tracked = m.TrackedNodes()
	require.Contains(tracked, n.ID)
	require.Equal(s2, tracked[n.ID].DrainStrategy)
}
