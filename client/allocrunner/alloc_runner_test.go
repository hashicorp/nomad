package allocrunner

import (
	"testing"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/state"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/stretchr/testify/require"
)

// TestAllocRunner_AllocState_Initialized asserts that getting TaskStates via
// AllocState() are initialized even before the AllocRunner has run.
func TestAllocRunner_AllocState_Initialized(t *testing.T) {
	t.Parallel()

	alloc := mock.Alloc()
	logger := testlog.HCLogger(t)

	conf := &Config{
		Alloc:            alloc,
		Logger:           logger,
		ClientConfig:     config.TestClientConfig(),
		StateDB:          state.NoopDB{},
		Consul:           nil,
		Vault:            nil,
		StateUpdater:     nil,
		PrevAllocWatcher: nil,
	}

	ar, err := NewAllocRunner(conf)
	require.NoError(t, err)

	allocState := ar.AllocState()

	require.NotNil(t, allocState)
	require.NotNil(t, allocState.TaskStates[alloc.Job.TaskGroups[0].Tasks[0].Name])
}

/*

import (
	"testing"

	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	clientconfig "github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

func testAllocRunnerFromAlloc(t *testing.T, alloc *structs.Allocation) *allocRunner {
	cconf := clientconfig.DefaultConfig()
	config := &Config{
		ClientConfig: cconf,
		Logger:       testlog.HCLogger(t).With("unit_test", t.Name()),
		Alloc:        alloc,
	}

	ar := NewAllocRunner(config)
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
*/
