// Copyright (c) HashiCorp, Inc.
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

// ModifyIndexTokenizer returns a tokenizer by ModifyIndex.
func ModifyIndexTokenizer[T modifyIndexGetter](target string) Tokenizer[T] {
	// attempt to convert token to uint for iterators ordered numerically.
	// it's safe to ignore the error here because the `next` method ignores
	// this field for string tokens and 0 is valid for an unset numeric token.
	targetIndex, _ := strconv.ParseUint(target, 10, 64)

	return func(item T) (string, int) {
		index := item.GetModifyIndex()
		token := fmt.Sprintf("%d", index)
		return token, cmp.Compare(index, targetIndex)
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
