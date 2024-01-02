// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package delayheap

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/stretchr/testify/require"
)

// HeapNodeImpl satisfies the HeapNode interface
type heapNodeImpl struct {
	dataObject interface{}
	id         string
	namespace  string
}

func (d *heapNodeImpl) Data() interface{} {
	return d.dataObject
}

func (d *heapNodeImpl) ID() string {
	return d.id
}

func (d *heapNodeImpl) Namespace() string {
	return d.namespace
}

func TestDelayHeap_PushPop(t *testing.T) {
	ci.Parallel(t)

	delayHeap := NewDelayHeap()
	now := time.Now()
	require := require.New(t)
	// a dummy type to use as the inner object in the heap
	type myObj struct {
		a int
		b string
	}
	dataNode1 := &heapNodeImpl{
		dataObject: &myObj{a: 0, b: "hey"},
		id:         "101",
		namespace:  "default",
	}
	delayHeap.Push(dataNode1, now.Add(-10*time.Minute))

	dataNode2 := &heapNodeImpl{
		dataObject: &myObj{a: 0, b: "hey"},
		id:         "102",
		namespace:  "default",
	}
	delayHeap.Push(dataNode2, now.Add(10*time.Minute))

	dataNode3 := &heapNodeImpl{
		dataObject: &myObj{a: 0, b: "hey"},
		id:         "103",
		namespace:  "default",
	}
	delayHeap.Push(dataNode3, now.Add(-15*time.Second))

	dataNode4 := &heapNodeImpl{
		dataObject: &myObj{a: 0, b: "hey"},
		id:         "101",
		namespace:  "test-namespace",
	}
	delayHeap.Push(dataNode4, now.Add(2*time.Hour))

	expectedWaitTimes := []time.Time{now.Add(-10 * time.Minute), now.Add(-15 * time.Second), now.Add(10 * time.Minute), now.Add(2 * time.Hour)}
	entries := getHeapEntries(delayHeap, now)
	for i, entry := range entries {
		require.Equal(expectedWaitTimes[i], entry.WaitUntil)
	}

}

func TestDelayHeap_Update(t *testing.T) {
	ci.Parallel(t)

	delayHeap := NewDelayHeap()
	now := time.Now()
	require := require.New(t)
	// a dummy type to use as the inner object in the heap
	type myObj struct {
		a int
		b string
	}
	dataNode1 := &heapNodeImpl{
		dataObject: &myObj{a: 0, b: "hey"},
		id:         "101",
		namespace:  "default",
	}
	delayHeap.Push(dataNode1, now.Add(-10*time.Minute))

	dataNode2 := &heapNodeImpl{
		dataObject: &myObj{a: 0, b: "hey"},
		id:         "102",
		namespace:  "default",
	}
	delayHeap.Push(dataNode2, now.Add(10*time.Minute))
	delayHeap.Update(dataNode1, now.Add(20*time.Minute))

	expectedWaitTimes := []time.Time{now.Add(10 * time.Minute), now.Add(20 * time.Minute)}
	expectedIdOrder := []string{"102", "101"}
	entries := getHeapEntries(delayHeap, now)
	for i, entry := range entries {
		require.Equal(expectedWaitTimes[i], entry.WaitUntil)
		require.Equal(expectedIdOrder[i], entry.Node.ID())
	}

}

func getHeapEntries(delayHeap *DelayHeap, now time.Time) []*delayHeapNode {
	var entries []*delayHeapNode
	for node := delayHeap.Pop(); node != nil; {
		entries = append(entries, node)
		node = delayHeap.Pop()
	}
	return entries
}
