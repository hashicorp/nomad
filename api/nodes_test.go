package api

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/testutil"
)

func TestNodes_List(t *testing.T) {
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	nodes := c.Nodes()

	var qm *QueryMeta
	var out []*Node
	var err error

	testutil.WaitForResult(func() (bool, error) {
		out, qm, err = nodes.List(nil)
		if err != nil {
			return false, err
		}
		if n := len(out); n != 1 {
			return false, fmt.Errorf("expected 1 node, got: %d", n)
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %s", err)
	})

	// Check that we got valid QueryMeta.
	assertQueryMeta(t, qm)
}

func TestNodes_Info(t *testing.T) {
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	nodes := c.Nodes()

	// Retrieving a non-existent node returns error
	_, _, err := nodes.Info("nope", nil)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error, got: %#v", err)
	}

	// Get the node ID
	var nodeID, dc string
	testutil.WaitForResult(func() (bool, error) {
		out, _, err := nodes.List(nil)
		if err != nil {
			return false, err
		}
		if n := len(out); n != 1 {
			return false, fmt.Errorf("expected 1 node, got: %d", n)
		}
		nodeID = out[0].ID
		dc = out[0].Datacenter
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %s", err)
	})

	// Querying for existing nodes returns properly
	result, qm, err := nodes.Info(nodeID, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	assertQueryMeta(t, qm)

	// Check that the result is what we expect
	if result.ID != nodeID || result.Datacenter != dc {
		t.Fatalf("expected %s (%s), got: %s (%s)",
			nodeID, dc,
			result.ID, result.Datacenter)
	}
}

func TestNodes_ToggleDrain(t *testing.T) {
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	nodes := c.Nodes()

	// Wait for node registration and get the ID
	var nodeID string
	testutil.WaitForResult(func() (bool, error) {
		out, _, err := nodes.List(nil)
		if err != nil {
			return false, err
		}
		if n := len(out); n != 1 {
			return false, fmt.Errorf("expected 1 node, got: %d", n)
		}
		nodeID = out[0].ID
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %s", err)
	})

	// Check for drain mode
	out, _, err := nodes.Info(nodeID, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if out.Drain {
		t.Fatalf("drain mode should be off")
	}

	// Toggle it on
	wm, err := nodes.ToggleDrain(nodeID, true, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	assertWriteMeta(t, wm)

	// Check again
	out, _, err = nodes.Info(nodeID, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if !out.Drain {
		t.Fatalf("drain mode should be on")
	}

	// Toggle off again
	wm, err = nodes.ToggleDrain(nodeID, false, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	assertWriteMeta(t, wm)

	// Check again
	out, _, err = nodes.Info(nodeID, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if out.Drain {
		t.Fatalf("drain mode should be off")
	}
}

func TestNodes_Allocations(t *testing.T) {
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	nodes := c.Nodes()

	// Looking up by a non-existent node returns nothing. We
	// don't check the index here because it's possible the node
	// has already registered, in which case we will get a non-
	// zero result anyways.
	allocs, _, err := nodes.Allocations("nope", nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if n := len(allocs); n != 0 {
		t.Fatalf("expected 0 allocs, got: %d", n)
	}
}

func TestNodes_ForceEvaluate(t *testing.T) {
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	nodes := c.Nodes()

	// Force-eval on a non-existent node fails
	_, _, err := nodes.ForceEvaluate("nope", nil)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error, got: %#v", err)
	}

	// Wait for node registration and get the ID
	var nodeID string
	testutil.WaitForResult(func() (bool, error) {
		out, _, err := nodes.List(nil)
		if err != nil {
			return false, err
		}
		if n := len(out); n != 1 {
			return false, fmt.Errorf("expected 1 node, got: %d", n)
		}
		nodeID = out[0].ID
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %s", err)
	})

	// Try force-eval again. We don't check the WriteMeta because
	// there are no allocations to process, so we would get an index
	// of zero. Same goes for the eval ID.
	_, _, err = nodes.ForceEvaluate(nodeID, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
}
