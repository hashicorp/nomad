package consul

import (
	"container/heap"
	"fmt"
	"time"

	cstructs "github.com/hashicorp/nomad/client/driver/structs"
)

type Check interface {
	Run() *cstructs.CheckResult
	ID() string
}

type consulCheck struct {
	check Check
	next  time.Time
	index int
}

type checkHeap struct {
	index map[string]*consulCheck
	heap  checksHeapImp
}

func NewConsulChecksHeap() *checkHeap {
	return &checkHeap{
		index: make(map[string]*consulCheck),
		heap:  make(checksHeapImp, 0),
	}
}

func (c *checkHeap) Push(check Check, next time.Time) error {
	if _, ok := c.index[check.ID()]; ok {
		return fmt.Errorf("check %v already exists", check.ID())
	}

	cCheck := &consulCheck{check, next, 0}

	c.index[check.ID()] = cCheck
	heap.Push(&c.heap, cCheck)
	return nil
}

func (c *checkHeap) Pop() *consulCheck {
	if len(c.heap) == 0 {
		return nil
	}

	cCheck := heap.Pop(&c.heap).(*consulCheck)
	delete(c.index, cCheck.check.ID())
	return cCheck
}

func (c *checkHeap) Peek() *consulCheck {
	if len(c.heap) == 0 {
		return nil
	}
	return c.heap[0]
}

func (c *checkHeap) Contains(check Check) bool {
	_, ok := c.index[check.ID()]
	return ok
}

func (c *checkHeap) Update(check Check, next time.Time) error {
	if cCheck, ok := c.index[check.ID()]; ok {
		cCheck.check = check
		cCheck.next = next
		heap.Fix(&c.heap, cCheck.index)
		return nil
	}

	return fmt.Errorf("heap doesn't contain check %v", check.ID())
}

func (c *checkHeap) Remove(id string) error {
	if cCheck, ok := c.index[id]; ok {
		heap.Remove(&c.heap, cCheck.index)
		delete(c.index, id)
		return nil
	}
	return fmt.Errorf("heap doesn't contain check %v", id)
}

func (c *checkHeap) Len() int { return len(c.heap) }

type checksHeapImp []*consulCheck

func (h checksHeapImp) Len() int { return len(h) }

func (h checksHeapImp) Less(i, j int) bool {
	// Two zero times should return false.
	// Otherwise, zero is "greater" than any other time.
	// (To sort it at the end of the list.)
	// Sort such that zero times are at the end of the list.
	iZero, jZero := h[i].next.IsZero(), h[j].next.IsZero()
	if iZero && jZero {
		return false
	} else if iZero {
		return false
	} else if jZero {
		return true
	}

	return h[i].next.Before(h[j].next)
}

func (h checksHeapImp) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *checksHeapImp) Push(x interface{}) {
	n := len(*h)
	check := x.(*consulCheck)
	check.index = n
	*h = append(*h, check)
}

func (h *checksHeapImp) Pop() interface{} {
	old := *h
	n := len(old)
	check := old[n-1]
	check.index = -1 // for safety
	*h = old[0 : n-1]
	return check
}
