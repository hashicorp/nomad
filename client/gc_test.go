package client

import (
	"testing"

	"github.com/hashicorp/nomad/nomad/mock"
)

func TestIndexedGCAllocPQ(t *testing.T) {
	pq := NewIndexedGCAllocPQ()

	_, ar1 := testAllocRunnerFromAlloc(mock.Alloc(), false)
	_, ar2 := testAllocRunnerFromAlloc(mock.Alloc(), false)
	_, ar3 := testAllocRunnerFromAlloc(mock.Alloc(), false)
	_, ar4 := testAllocRunnerFromAlloc(mock.Alloc(), false)

	pq.Push(ar1)
	pq.Push(ar2)
	pq.Push(ar3)
	pq.Push(ar4)

	allocID := pq.Pop().alloc.Alloc().ID
	if allocID != ar1.Alloc().ID {
		t.Fatalf("expected alloc %v, got %v", allocID, ar1.Alloc().ID)
	}

	allocID = pq.Pop().alloc.Alloc().ID
	if allocID != ar2.Alloc().ID {
		t.Fatalf("expected alloc %v, got %v", allocID, ar1.Alloc().ID)
	}

	allocID = pq.Pop().alloc.Alloc().ID
	if allocID != ar3.Alloc().ID {
		t.Fatalf("expected alloc %v, got %v", allocID, ar1.Alloc().ID)
	}

	allocID = pq.Pop().alloc.Alloc().ID
	if allocID != ar4.Alloc().ID {
		t.Fatalf("expected alloc %v, got %v", allocID, ar1.Alloc().ID)
	}

	gcAlloc := pq.Pop()
	if gcAlloc != nil {
		t.Fatalf("expected nil, got %v", gcAlloc)
	}
}
