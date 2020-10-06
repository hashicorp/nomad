package state

import (
	"errors"
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

// TestStateStore_Events_OnEvict tests that events in the state stores
// event publisher and go-memdb are evicted together when the event buffer
// size reaches its max.
func TestStateStore_Events_OnEvict(t *testing.T) {
	t.Parallel()

	cfg := &StateStoreConfig{
		Logger:            testlog.HCLogger(t),
		Region:            "global",
		EnablePublisher:   true,
		EventBufferSize:   10,
		DurableEventCount: 10,
	}
	s := TestStateStoreCfg(t, cfg)

	_, err := s.EventPublisher()
	require.NoError(t, err)

	// force 3 evictions
	for i := 1; i < 13; i++ {
		require.NoError(t,
			s.UpsertNodeMsgType(structs.NodeRegisterRequestType, uint64(i), mock.Node()),
		)
	}

	get := func() []*structs.Events {
		var out []*structs.Events
		iter, err := s.Events(nil)
		require.NoError(t, err)
		for {
			raw := iter.Next()
			if raw == nil {
				break
			}
			e := raw.(*structs.Events)

			out = append(out, e)
		}
		return out
	}

	// event publisher is async so wait for it to prune
	testutil.WaitForResult(func() (bool, error) {
		out := get()
		if len(out) != 10 {
			return false, errors.New("Expected event count to be pruned to 10")
		}
		return true, nil
	}, func(err error) {
		require.Fail(t, err.Error())
		t.Fatalf("err: %s", err)
	})

	out := get()
	require.Equal(t, 3, int(out[0].Index))

}

// TestStateStore_Events_OnEvict_Missing tests behavior when the event publisher
// evicts an event and there is no corresponding go-memdb entry due to durability
// settings
func TestStateStore_Events_OnEvict_Missing(t *testing.T) {
	t.Parallel()

	cfg := &StateStoreConfig{
		Logger:            testlog.HCLogger(t),
		Region:            "global",
		EnablePublisher:   true,
		EventBufferSize:   10,
		DurableEventCount: 0,
	}
	s := TestStateStoreCfg(t, cfg)

	_, err := s.EventPublisher()
	require.NoError(t, err)

	getEvents := func() []*structs.Events {
		var out []*structs.Events
		iter, err := s.Events(nil)
		require.NoError(t, err)
		for {
			raw := iter.Next()
			if raw == nil {
				break
			}
			e := raw.(*structs.Events)

			out = append(out, e)
		}
		return out
	}

	// Publish 13 events to fill buffer and force 3 evictions
	for i := 1; i < 13; i++ {
		require.NoError(t,
			s.UpsertNodeMsgType(structs.NodeRegisterRequestType, uint64(i), mock.Node()),
		)
	}

	// event publisher is async so wait for it to prune
	testutil.WaitForResult(func() (bool, error) {
		out := getEvents()
		if len(out) != 0 {
			return false, fmt.Errorf("Expected event count to be %d, got: %d", 0, len(out))
		}
		return true, nil
	}, func(err error) {
		require.Fail(t, err.Error())
		t.Fatalf("err: %s", err)
	})
}
