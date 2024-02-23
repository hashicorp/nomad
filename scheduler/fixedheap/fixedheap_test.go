package fixedheap

import (
	"testing"

	"github.com/shoenig/test/must"
)

func TestFixedHeap(t *testing.T) {
	heap := NewFixedHeap[uint64](5, func(a, b uint64) bool { return a < b })

	pushCases := []struct {
		in         uint64
		expectOk   bool
		expectOut  uint64
		expectPeek uint64
	}{
		{2, true, 2, 2},
		{0, true, 0, 0},
		{3, true, 3, 0},
		{5, true, 5, 0},
		{4, true, 4, 0},
		{1, false, 0, 1},
	}

	for _, tc := range pushCases {
		out, ok := heap.Push(tc.in)
		must.Eq(t, tc.expectOk, ok, must.Sprint("expected ok"))
		must.Eq(t, tc.expectOut, out, must.Sprint("expected out"))
		peeked, ok := heap.Peek()
		must.True(t, ok)
		must.Eq(t, tc.expectPeek, peeked, must.Sprint("expected Peek()"))
	}

	must.Eq(t, []uint64{5, 4, 3, 2, 1}, heap.Slice())
	must.Eq(t, []uint64{1, 2, 3, 4, 5}, heap.SliceReverse())

	peekNCases := []struct {
		index      int
		expectPeek uint64
		expectOk   bool
	}{
		{0, 5, true},
		{1, 4, true},
		{2, 3, true},
		{3, 2, true},
		{4, 1, true},
		{5, 0, false},
	}
	for _, tc := range peekNCases {
		out, ok := heap.PeekN(tc.index)
		must.Eq(t, tc.expectOk, ok, must.Sprintf("at PeekN(%d)", tc.index))
		must.Eq(t, tc.expectPeek, out, must.Sprintf("at PeekN(%d)", tc.index))
	}

	popCases := []struct {
		expectPop uint64
		expectOk  bool
	}{
		{1, true},
		{2, true},
		{3, true},
		{4, true},
		{5, true},
		{0, false},
	}
	for _, tc := range popCases {
		out, ok := heap.Pop()
		must.Eq(t, tc.expectOk, ok)
		must.Eq(t, tc.expectPop, out)
	}
}
