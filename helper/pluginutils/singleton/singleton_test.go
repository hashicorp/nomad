// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package singleton

import (
	"fmt"
	"sync"
	"testing"
	"time"

	log "github.com/hashicorp/go-hclog"
	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pluginutils/loader"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/stretchr/testify/require"
)

func harness(t *testing.T) (*SingletonLoader, *loader.MockCatalog) {
	c := &loader.MockCatalog{}
	s := NewSingletonLoader(testlog.HCLogger(t), c)
	return s, c
}

// Test that multiple dispenses return the same instance
func TestSingleton_Dispense(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	dispenseCalled := 0
	s, c := harness(t)
	c.DispenseF = func(_, _ string, _ *base.AgentConfig, _ log.Logger) (loader.PluginInstance, error) {
		p := &base.MockPlugin{}
		i := &loader.MockInstance{
			ExitedF: func() bool { return false },
			PluginF: func() interface{} { return p },
		}
		dispenseCalled++
		return i, nil
	}

	// Retrieve the plugin many times in parallel
	const count = 128
	var l sync.Mutex
	var wg sync.WaitGroup
	plugins := make(map[interface{}]struct{}, 1)
	waitCh := make(chan struct{})
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func() {
			// Wait for unblock
			<-waitCh

			// Retrieve the plugin
			p1, err := s.Dispense("foo", "bar", nil, testlog.HCLogger(t))
			require.NotNil(p1)
			require.NoError(err)
			i1 := p1.Plugin()
			require.NotNil(i1)
			l.Lock()
			plugins[i1] = struct{}{}
			l.Unlock()
			wg.Done()
		}()
	}
	time.Sleep(10 * time.Millisecond)
	close(waitCh)
	wg.Wait()
	require.Len(plugins, 1)
	require.Equal(1, dispenseCalled)
}

// Test that after a plugin is dispensed, if it exits, an error is returned on
// the next dispense
func TestSingleton_Dispense_Exit_Dispense(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	exited := false
	dispenseCalled := 0
	s, c := harness(t)
	c.DispenseF = func(_, _ string, _ *base.AgentConfig, _ log.Logger) (loader.PluginInstance, error) {
		p := &base.MockPlugin{}
		i := &loader.MockInstance{
			ExitedF: func() bool { return exited },
			PluginF: func() interface{} { return p },
		}
		dispenseCalled++
		return i, nil
	}

	// Retrieve the plugin
	logger := testlog.HCLogger(t)
	p1, err := s.Dispense("foo", "bar", nil, logger)
	require.NotNil(p1)
	require.NoError(err)

	i1 := p1.Plugin()
	require.NotNil(i1)
	require.Equal(1, dispenseCalled)

	// Mark the plugin as exited and retrieve again
	exited = true
	_, err = s.Dispense("foo", "bar", nil, logger)
	require.Error(err)
	require.Contains(err.Error(), "exited")
	require.Equal(1, dispenseCalled)

	// Mark the plugin as non-exited and retrieve again
	exited = false
	p2, err := s.Dispense("foo", "bar", nil, logger)
	require.NotNil(p2)
	require.NoError(err)
	require.Equal(2, dispenseCalled)

	i2 := p2.Plugin()
	require.NotNil(i2)
	if i1 == i2 {
		t.Fatalf("i1 and i2 shouldn't be the same instance: %p vs %p", i1, i2)
	}
}

// Test that if a plugin errors while being dispensed, the error is returned but
// not saved
func TestSingleton_DispenseError_Dispense(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	dispenseCalled := 0
	good := func(_, _ string, _ *base.AgentConfig, _ log.Logger) (loader.PluginInstance, error) {
		p := &base.MockPlugin{}
		i := &loader.MockInstance{
			ExitedF: func() bool { return false },
			PluginF: func() interface{} { return p },
		}
		dispenseCalled++
		return i, nil
	}

	bad := func(_, _ string, _ *base.AgentConfig, _ log.Logger) (loader.PluginInstance, error) {
		dispenseCalled++
		return nil, fmt.Errorf("bad")
	}

	s, c := harness(t)
	c.DispenseF = bad

	// Retrieve the plugin
	logger := testlog.HCLogger(t)
	p1, err := s.Dispense("foo", "bar", nil, logger)
	require.Nil(p1)
	require.Error(err)
	require.Equal(1, dispenseCalled)

	// Dispense again and ensure the same error isn't saved
	c.DispenseF = good
	p2, err := s.Dispense("foo", "bar", nil, logger)
	require.NotNil(p2)
	require.NoError(err)
	require.Equal(2, dispenseCalled)

	i2 := p2.Plugin()
	require.NotNil(i2)
}

// Test that if a plugin errors while being reattached, the error is returned but
// not saved
func TestSingleton_ReattachError_Dispense(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	dispenseCalled, reattachCalled := 0, 0
	s, c := harness(t)
	c.DispenseF = func(_, _ string, _ *base.AgentConfig, _ log.Logger) (loader.PluginInstance, error) {
		p := &base.MockPlugin{}
		i := &loader.MockInstance{
			ExitedF: func() bool { return false },
			PluginF: func() interface{} { return p },
		}
		dispenseCalled++
		return i, nil
	}
	c.ReattachF = func(_, _ string, _ *plugin.ReattachConfig) (loader.PluginInstance, error) {
		reattachCalled++
		return nil, fmt.Errorf("bad")
	}

	// Retrieve the plugin
	logger := testlog.HCLogger(t)
	p1, err := s.Reattach("foo", "bar", nil)
	require.Nil(p1)
	require.Error(err)
	require.Equal(0, dispenseCalled)
	require.Equal(1, reattachCalled)

	// Dispense and ensure the same error isn't saved
	p2, err := s.Dispense("foo", "bar", nil, logger)
	require.NotNil(p2)
	require.NoError(err)
	require.Equal(1, dispenseCalled)
	require.Equal(1, reattachCalled)

	i2 := p2.Plugin()
	require.NotNil(i2)
}

// Test that after reattaching, dispense returns the same instance
func TestSingleton_Reattach_Dispense(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	dispenseCalled, reattachCalled := 0, 0
	s, c := harness(t)
	c.DispenseF = func(_, _ string, _ *base.AgentConfig, _ log.Logger) (loader.PluginInstance, error) {
		dispenseCalled++
		return nil, fmt.Errorf("bad")
	}
	c.ReattachF = func(_, _ string, _ *plugin.ReattachConfig) (loader.PluginInstance, error) {
		p := &base.MockPlugin{}
		i := &loader.MockInstance{
			ExitedF: func() bool { return false },
			PluginF: func() interface{} { return p },
		}
		reattachCalled++
		return i, nil
	}

	// Retrieve the plugin
	logger := testlog.HCLogger(t)
	p1, err := s.Reattach("foo", "bar", nil)
	require.NotNil(p1)
	require.NoError(err)
	require.Equal(0, dispenseCalled)
	require.Equal(1, reattachCalled)

	i1 := p1.Plugin()
	require.NotNil(i1)

	// Dispense and ensure the same instance returned
	p2, err := s.Dispense("foo", "bar", nil, logger)
	require.NotNil(p2)
	require.NoError(err)
	require.Equal(0, dispenseCalled)
	require.Equal(1, reattachCalled)

	i2 := p2.Plugin()
	require.NotNil(i2)
	if i1 != i2 {
		t.Fatalf("i1 and i2 should be the same instance: %p vs %p", i1, i2)
	}
}
