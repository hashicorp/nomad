package allocrunnerv2

import (
	"context"
	"testing"

	"github.com/hashicorp/nomad/client/allocrunnerv2/config"
	"github.com/hashicorp/nomad/client/allocrunnerv2/interfaces"
	clientconfig "github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

func testAllocRunnerFromAlloc(t *testing.T, alloc *structs.Allocation) *allocRunner {
	cconf := clientconfig.DefaultConfig()
	config := &config.Config{
		ClientConfig: cconf,
		Logger:       testlog.HCLogger(t).With("unit_test", t.Name()),
		Allocation:   alloc,
	}

	ar, err := NewAllocRunner(context.Background(), config)
	if err != nil {
		t.Fatalf("Failed to create test alloc runner: %v", err)
	}

	return ar

}

func testAllocRunner(t *testing.T) *allocRunner {
	return testAllocRunnerFromAlloc(t, mock.Alloc())
}

// preRun is a test RunnerHook that captures whether Prerun was called on it
type preRun struct{ run bool }

func (p *preRun) Name() string { return "pre" }
func (p *preRun) Prerun() error {
	p.run = true
	return nil
}

// postRun is a test RunnerHook that captures whether Postrun was called on it
type postRun struct{ run bool }

func (p *postRun) Name() string { return "post" }
func (p *postRun) Postrun() error {
	p.run = true
	return nil
}

// Tests that prerun only runs pre run hooks.
func TestAllocRunner_Prerun_Basic(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	ar := testAllocRunner(t)

	// Overwrite the hooks with test hooks
	pre := &preRun{}
	post := &postRun{}
	ar.runnerHooks = []interfaces.RunnerHook{pre, post}

	// Run the hooks
	require.NoError(ar.prerun())

	// Assert only the pre is run
	require.True(pre.run)
	require.False(post.run)
}

// Tests that postrun only runs post run hooks.
func TestAllocRunner_Postrun_Basic(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	ar := testAllocRunner(t)

	// Overwrite the hooks with test hooks
	pre := &preRun{}
	post := &postRun{}
	ar.runnerHooks = []interfaces.RunnerHook{pre, post}

	// Run the hooks
	require.NoError(ar.postrun())

	// Assert only the pre is run
	require.True(post.run)
	require.False(pre.run)
}
