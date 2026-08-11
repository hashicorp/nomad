package btree

import (
	"slices"
	"sort"
)

// BpTree is a generic implementation of a B+ tree.
//
// This implementation is more memory efficient than TreeSet
// and allows for fast and low allocation querying of stored
// items using the Range method.
//
// It is not yet complete, missing Find and Delete implementations.
type BpTree[T BpTreeItem[T]] struct {
	root         *node[T]
	nodeCapacity int
}

type BpTreeItem[K any] interface {
	LessThan(other K) bool
}

func NewBpTree[T BpTreeItem[T]](size int) *BpTree[T] {
	return &BpTree[T]{
		root: &node[T]{
			isLeaf: true,
			items:  make([]T, 0, size),
		},
		nodeCapacity: size,
	}
}

type node[T any] struct {
	isLeaf bool
	size   int
	maxKey T

	// fields specific to internal nodes
	children  []*node[T]
	totalSize int

	// fields specific to leaf nodes
	items    []T
	nextLeaf *node[T]
	prevLeaf *node[T]
}

func (b *BpTree[T]) Insert(item T) {
	if b.root.size == b.nodeCapacity {
		newRoot := &node[T]{
			isLeaf:   false,
			children: []*node[T]{b.root},
			maxKey:   b.root.maxKey,
		}
		newRoot.children = append(newRoot.children, b.root)
		b.splitNode(newRoot, 0)
		b.root = newRoot
	}
	b.root.totalSize += 1

	b.insert(item, b.root)
}

// Unimplemented
func (b *BpTree[T]) Delete(item BpTreeItem[T]) {}

// Unimplemented
func (b *BpTree[T]) Find(key BpTreeItem[T]) BpTreeItem[T] { return nil }

func (b *BpTree[T]) Range(start, end int) []T {
	leaf, offset := b.findLeaf(b.root, start)
	if leaf == nil {
		return nil
	}

	res := []T{}

	for leaf != nil {
		for i := offset; i < leaf.size; i++ {
			res = append(res, leaf.items[i])

			if len(res) == (end - start) {
				return res
			}
		}
		leaf = leaf.nextLeaf
		offset = 0
	}

	return res
}

func (b *BpTree[T]) insert(item T, n *node[T]) {
	if n.isLeaf {
		idx := sort.Search(n.size, func(i int) bool {
			return item.LessThan(n.items[i])
		})

		n.items = slices.Insert(n.items, idx, item)

		if idx == n.size {
			n.maxKey = item
		}
		n.size++
	} else {
		idx := sort.Search(n.size, func(i int) bool {
			return item.LessThan(n.children[i].maxKey)
		})
		// if the search idx == n.size, this will be the new
		// max of a leaf, so decrement idx to insert in the
		// largest current child node.
		if idx == n.size {
			idx = n.size - 1
		}

		if n.children[idx].size == b.nodeCapacity {
			b.splitNode(n, idx)
			if !item.LessThan(n.children[idx].maxKey) {
				idx++
			}
		}

		b.insert(item, n.children[idx])

		n.totalSize += 1

		if idx == n.size-1 {
			n.maxKey = n.children[idx].maxKey
		}
	}
}

func (b *BpTree[T]) findLeaf(n *node[T], idx int) (*node[T], int) {
	// This shouldn't happen
	if n == nil || idx < 0 {
		return nil, 0
	}

	if n.isLeaf {
		if idx >= n.size {
			// the request start is not within the tree
			return nil, 0
		}
		return n, idx
	}

	offset := idx
	for _, c := range n.children {
		size := 0
		if c.isLeaf {
			size += c.size
		} else {
			size += c.totalSize
		}
		if offset-size <= 0 {
			return b.findLeaf(c, offset)
		}
		offset -= size
	}

	return nil, 0
}

func (b *BpTree[T]) splitNode(parent *node[T], index int) {
	nodeToSplit := parent.children[index]
	remain := nodeToSplit.size % 2
	mid := nodeToSplit.size / 2

	newNode := &node[T]{
		isLeaf: nodeToSplit.isLeaf,
		size:   mid + remain,
	}

	// This is ugly, but in general it allocates a slice of items or children.
	// It copies over the items/children from the splitting node given the mid point,
	// then sets some leaf/internal node specific data.
	if nodeToSplit.isLeaf {
		newNode.items = make([]T, mid+remain, b.nodeCapacity)
		copy(newNode.items, nodeToSplit.items[mid:])
		newNode.maxKey = newNode.items[newNode.size-1]

		// update leaf pointers
		newNode.nextLeaf = nodeToSplit.nextLeaf
		newNode.prevLeaf = nodeToSplit
		nodeToSplit.nextLeaf = newNode
		if newNode.nextLeaf != nil {
			newNode.nextLeaf.prevLeaf = newNode
		}

		// TODO: I think reslicing is safe here (same with below)
		// but need to make sure this doesn't leak anything
		nodeToSplit.size = mid
		nodeToSplit.items = nodeToSplit.items[:mid]
		nodeToSplit.maxKey = nodeToSplit.items[nodeToSplit.size-1]
	} else {
		newNode.children = make([]*node[T], mid+remain, b.nodeCapacity)
		copy(newNode.children, nodeToSplit.children[mid:])
		newNode.maxKey = newNode.children[newNode.size-1].maxKey

		nodeToSplit.size = mid
		nodeToSplit.children = nodeToSplit.children[:mid]
		nodeToSplit.maxKey = nodeToSplit.children[nodeToSplit.size-1].maxKey

		// accumulate new totalSize for both nodes
		nodeToSplit.totalSize = 0
		for _, c := range nodeToSplit.children {
			if c.isLeaf {
				nodeToSplit.totalSize += c.size
			} else {
				nodeToSplit.totalSize += c.totalSize
			}
		}
		for _, c := range newNode.children {
			if c.isLeaf {
				newNode.totalSize += c.size
			} else {
				newNode.totalSize += c.totalSize
			}
		}
	}

	// Insert and update the parent node with the new child
	parent.children = slices.Insert(parent.children, index+1, newNode)
	// If we split the right most node, update the maxKey
	if index == parent.size-1 {
		parent.maxKey = newNode.maxKey
	}
	parent.size++
}
