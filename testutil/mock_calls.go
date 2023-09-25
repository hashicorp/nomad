// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package testutil

import (
	"maps"
	"sync"

	"github.com/mitchellh/go-testing-interface"
)

func NewCallCounter() *CallCounter {
	return &CallCounter{
		counts: make(map[string]int),
	}
}

type CallCounter struct {
	lock   sync.Mutex
	counts map[string]int
}

func (c *CallCounter) Inc(name string) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.counts[name]++
}

func (c *CallCounter) Get() map[string]int {
	c.lock.Lock()
	defer c.lock.Unlock()
	return maps.Clone(c.counts)
}

func (c *CallCounter) AssertCalled(t testing.T, name string) {
	t.Helper()
	counts := c.Get()
	if _, ok := counts[name]; !ok {
		t.Errorf("'%s' not called; all counts: %v", counts)
	}
}
