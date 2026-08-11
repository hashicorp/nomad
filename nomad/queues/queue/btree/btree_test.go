package btree

import (
	"testing"

	"github.com/shoenig/test/must"
)

type cint int

func (c cint) LessThan(o cint) bool {
	return c < o
}

func TestBptree_splitNode(t *testing.T) {
	t.Run("splits odd sized leaf node", func(t *testing.T) {
		testNode := &node[cint]{
			isLeaf: false,
			size:   1,
			children: []*node[cint]{
				{
					isLeaf: true,
					size:   3,
					items: []cint{
						1, 2, 3,
					},
				},
			},
		}

		testTree := NewBpTree[cint](3)
		testTree.root = testNode

		testTree.splitNode(testNode, 0)

		must.Len(t, 2, testTree.root.children)
		// Assert correct nodes exist
		must.Len(t, 1, testTree.root.children[0].items)
		must.Len(t, 2, testTree.root.children[1].items)

		// Assert original node has correct items
		must.Eq(t, 1, testTree.root.children[0].items[0])

		// Assert new node has correct items
		must.Eq(t, 2, testTree.root.children[1].items[0])
		must.Eq(t, 3, testTree.root.children[1].items[1])
	})

	t.Run("splits even sized leaf node", func(t *testing.T) {
		testNode := &node[cint]{
			isLeaf: false,
			size:   1,
			children: []*node[cint]{
				{
					isLeaf: true,
					size:   2,
					items: []cint{
						1, 2,
					},
				},
			},
		}

		testTree := NewBpTree[cint](2)
		testTree.root = testNode

		testTree.splitNode(testNode, 0)

		must.Len(t, 2, testTree.root.children)
		// Assert correct nodes exist
		must.Len(t, 1, testTree.root.children[0].items)
		must.Len(t, 1, testTree.root.children[1].items)

		// Assert original node has correct items
		must.Eq(t, 1, testTree.root.children[0].items[0])

		// Assert new node has correct items
		must.Eq(t, 2, testTree.root.children[1].items[0])
	})

	t.Run("splits odd sized internal node", func(t *testing.T) {
		testNode := &node[cint]{
			isLeaf: false,
			size:   1,
			children: []*node[cint]{
				{
					isLeaf: false,
					size:   3,
					children: []*node[cint]{
						{isLeaf: true, size: 1, items: []cint{1}, maxKey: 1},
						{isLeaf: true, size: 1, items: []cint{2}, maxKey: 2},
						{isLeaf: true, size: 1, items: []cint{3}, maxKey: 3},
					},
					maxKey:    3,
					totalSize: 3,
				},
			},
		}

		testTree := NewBpTree[cint](3)
		testTree.root = testNode

		testTree.splitNode(testNode, 0)

		must.Len(t, 2, testTree.root.children)
		// Assert correct nodes exist
		must.Len(t, 1, testTree.root.children[0].children)
		must.Len(t, 2, testTree.root.children[1].children)

		// Assert original node has correct children
		must.Eq(t, cint(1), testTree.root.children[0].children[0].maxKey)

		// Assert new node has correct children
		must.Eq(t, cint(2), testTree.root.children[1].children[0].maxKey)
		must.Eq(t, cint(3), testTree.root.children[1].children[1].maxKey)

		// Assert totalSize is correct
		must.Eq(t, 1, testTree.root.children[0].totalSize)
		must.Eq(t, 2, testTree.root.children[1].totalSize)
	})

	t.Run("splits even sized internal node", func(t *testing.T) {
		testNode := &node[cint]{
			isLeaf: false,
			size:   1,
			children: []*node[cint]{
				{
					isLeaf: false,
					size:   2,
					children: []*node[cint]{
						{isLeaf: true, size: 1, items: []cint{1}, maxKey: 1},
						{isLeaf: true, size: 1, items: []cint{2}, maxKey: 2},
					},
					maxKey:    2,
					totalSize: 2,
				},
			},
		}

		testTree := NewBpTree[cint](2)
		testTree.root = testNode

		testTree.splitNode(testNode, 0)

		must.Len(t, 2, testTree.root.children)
		// Assert correct nodes exist
		must.Len(t, 1, testTree.root.children[0].children)
		must.Len(t, 1, testTree.root.children[1].children)

		// Assert original node has correct children
		must.Eq(t, cint(1), testTree.root.children[0].children[0].maxKey)

		// Assert new node has correct children
		must.Eq(t, cint(2), testTree.root.children[1].children[0].maxKey)

		// Assert totalSize is correct
		must.Eq(t, 1, testTree.root.children[0].totalSize)
		must.Eq(t, 1, testTree.root.children[1].totalSize)
	})
}
