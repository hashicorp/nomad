package tasklifecycle

import (
	"testing"

	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
)

func requireTaskBlocked(t *testing.T, c *Coordinator, task *structs.Task) {
	ch := c.StartConditionForTask(task)
	requireChannelBlocking(t, ch, task.Name)
}

func requireTaskAllowed(t *testing.T, c *Coordinator, task *structs.Task) {
	ch := c.StartConditionForTask(task)
	requireChannelPassing(t, ch, task.Name)
}

func requireChannelPassing(t *testing.T, ch <-chan struct{}, name string) {
	testutil.WaitForResult(func() (bool, error) {
		return !isChannelBlocking(ch), nil
	}, func(_ error) {
		t.Fatalf("%s channel was blocking, should be passing", name)
	})
}

func requireChannelBlocking(t *testing.T, ch <-chan struct{}, name string) {
	testutil.WaitForResult(func() (bool, error) {
		return isChannelBlocking(ch), nil
	}, func(_ error) {
		t.Fatalf("%s channel was passing, should be blocking", name)
	})
}

func isChannelBlocking(ch <-chan struct{}) bool {
	select {
	case <-ch:
		return false
	default:
		return true
	}
}
