package pluginmanager

import (
	"sync"
	"testing"

	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/stretchr/testify/require"
)

func TestPluginGroup_RegisterAndRun(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	var hasRun bool
	var wg sync.WaitGroup
	wg.Add(1)
	mananger := &MockPluginManager{RunF: func() {
		hasRun = true
		wg.Done()
	}}

	group := New(testlog.HCLogger(t))
	require.NoError(group.RegisterAndRun(mananger))
	wg.Wait()
	require.True(hasRun)
}

func TestPluginGroup_Shutdown(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	var stack []int
	var stackMu sync.Mutex
	var runWg sync.WaitGroup
	var shutdownWg sync.WaitGroup
	group := New(testlog.HCLogger(t))
	for i := 1; i < 4; i++ {
		i := i
		runWg.Add(1)
		shutdownWg.Add(1)
		mananger := &MockPluginManager{RunF: func() {
			stackMu.Lock()
			defer stackMu.Unlock()
			defer runWg.Done()
			stack = append(stack, i)
		}, ShutdownF: func() {
			stackMu.Lock()
			defer stackMu.Unlock()
			defer shutdownWg.Done()
			idx := len(stack) - 1
			val := stack[idx]
			require.Equal(i, val)
			stack = stack[:idx]
		}}
		require.NoError(group.RegisterAndRun(mananger))
		runWg.Wait()
	}
	group.Shutdown()
	shutdownWg.Wait()
	require.Empty(stack)

	require.Error(group.RegisterAndRun(&MockPluginManager{}))
}
