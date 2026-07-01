// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package paginator

import (
	"cmp"
	"fmt"
	"strconv"
	"strings"
)

// Tokenizer is the interface that must be implemented to provide pagination
// tokens to the Paginator. It returns the token extracted from an object and
// the results of a comparison against the target token the tokenizer is
// seeking. Implementations should close over the token we're seeking.
type Tokenizer[T any] func(item T) (string, int)

// NamespaceIDTokenizer returns a tokenizer by Namespace and ID.
func NamespaceIDTokenizer[T namespaceIDGetter](target string) Tokenizer[T] {
	return func(item T) (string, int) {
		ns := item.GetNamespace()
		id := item.GetID()

		// Use a character that is not part of validNamespaceName as separator to
		// avoid accidentally generating collisions.
		// For example, namespace `a` and job `b-c` would collide with namespace
		// `a-b` and job `c` into the same token `a-b-c`, since `-` is an allowed
		// character in namespace.
		token := fmt.Sprintf("%s.%s", ns, id)
		return token, cmp.Compare(token, target)
	}
}

// IDTokenizer returns a tokenizer by ID.
func IDTokenizer[T idGetter](target string) Tokenizer[T] {
	return func(item T) (string, int) {
		id := item.GetID()
		return id, cmp.Compare(id, target)
	}
}

// CreateIndexAndIDTokenizer returns a tokenizer by CreateIndex and ID.
func CreateIndexAndIDTokenizer[T idAndCreateIndexGetter](target string) Tokenizer[T] {
	return func(item T) (string, int) {
		index := item.GetCreateIndex()
		id := item.GetID()
		token := fmt.Sprintf("%d.%s", index, id)

		// Split the target to extract the create index and the ID values.
		targetParts := strings.SplitN(target, ".", 2)
		// If the target wasn't composed of both parts, directly compare.
		if len(targetParts) < 2 {
			return token, cmp.Compare(token, target)
		}

		// Convert the create index to an integer for comparison. This
		// prevents a lexigraphical comparison of the create index which
		// can cause unexpected results when comparing index values like
		// '12' and '102'. If the index cannot be converted to an integer,
		// fall back to direct comparison.
		targetIndex, err := strconv.Atoi(targetParts[0])
		if err != nil {
			return token, cmp.Compare(token, target)
		}

		indexCmp := cmp.Compare(index, uint64(targetIndex))
		if indexCmp != 0 {
			return token, indexCmp
		}

		// If the index values are equivalent use the ID values
		// as the comparison.
		return token, cmp.Compare(id, targetParts[1])
	}
}

// ModifyIndexAndNamespaceIDTokenizer returns a tokenizer by ModifyIndex, with
// Namespace and ID as a tiebreaker. ModifyIndex is not unique across objects
// (several may be written in one Raft transaction), so ModifyIndex alone does
// not identify a unique position to resume pagination from. Namespace and ID
// make the token a total order that matches the memdb iteration order of the
// non-unique modify_index index, which breaks ties on the (Namespace, ID)
// primary key.
func ModifyIndexAndNamespaceIDTokenizer[T modifyIndexAndNamespaceIDGetter](target string) Tokenizer[T] {
	return func(item T) (string, int) {
		index := item.GetModifyIndex()
		ns := item.GetNamespace()
		id := item.GetID()
		token := fmt.Sprintf("%d.%s.%s", index, ns, id)

		// Namespace cannot contain '.', so the index and namespace are the first
		// two segments; the ID (which may contain '.') is the remainder.
		parts := strings.SplitN(target, ".", 3)

		// Parse the index numerically so values like 12 and 102 compare
		// correctly rather than lexicographically. A target that doesn't begin
		// with an integer is malformed; fall back to a direct string compare.
		targetIndex, err := strconv.ParseUint(parts[0], 10, 64)
		if err != nil {
			return token, cmp.Compare(token, target)
		}
		if c := cmp.Compare(index, targetIndex); c != 0 {
			return token, c
		}

		// Indexes are equal; break the tie by namespace then ID. A target with
		// fewer segments (e.g. a legacy bare-integer token from before this
		// tiebreaker existed, seen during a rolling upgrade) compares only the
		// segments it has, degrading to the previous index-only behavior.
		if len(parts) < 2 {
			return token, 0
		}
		if c := cmp.Compare(ns, parts[1]); c != 0 {
			return token, c
		}
		if len(parts) < 3 {
			return token, 0
		}
		return token, cmp.Compare(id, parts[2])
	}
}

// namespaceIDGetter must be implemented by structs that want to use
// Namespace and ID as their pagination token.
type namespaceIDGetter interface {
	GetNamespace() string
	GetID() string
}

// idGetter must be implemented by structs that want to use their ID (without
// namespace) as their pagination token.
type idGetter interface {
	GetID() string
}

// namespaceGetter must be implemented by structs that want to use Namespace
// alone as their pagination token.
type namespaceGetter interface {
	GetNamespace() string
}

// idAndCreateIndexGetter must be implemented by structs that want to use
// CreateIndex and ID as their pagination token.
type idAndCreateIndexGetter interface {
	GetID() string
	GetCreateIndex() uint64
}

// modifyIndexGetter must be implemented by structs that want to use ModifyIndex
// as their pagination token.
type modifyIndexGetter interface {
	GetModifyIndex() uint64
}

// modifyIndexAndNamespaceIDGetter must be implemented by structs that want to
// use ModifyIndex with a Namespace and ID tiebreaker as their pagination token.
type modifyIndexAndNamespaceIDGetter interface {
	modifyIndexGetter
	namespaceIDGetter
}
