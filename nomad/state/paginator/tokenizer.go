// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package paginator

import (
	"cmp"
	"strconv"
	"strings"
)

// Tokenizer is the interface that must be implemented to provide pagination
// tokens to the Paginator. It returns the token extracted from an object and
// the results of a comparison against the target token the tokenizer is
// seeking. Implementations should close over the token we're seeking.
type Tokenizer[T any] func(item T) (string, int)

// tokenField is one component of a pagination token, compared numerically
// (12 before 102) or as a string.
type tokenField struct {
	str       string
	num       uint64
	isNumeric bool
}

// stringField builds a lexicographically-compared token component.
func stringField(s string) tokenField { return tokenField{str: s} }

// numericField builds a numerically-compared token component.
func numericField(n uint64) tokenField { return tokenField{num: n, isNumeric: true} }

// tokenAndCompare joins fields into a '.'-separated token and compares it to
// target field by field, which matches memdb's compound index order. Comparing
// the joined strings does not: '.' > '-' puts "team-a.x" before "team.x".
// Only the last field may contain '.'. Missing target segments compare equal,
// and a non-integer numeric segment returns ok == false.
func tokenAndCompare(fields []tokenField, target string) (token string, c int, ok bool) {
	parts := make([]string, len(fields))
	for i, f := range fields {
		if f.isNumeric {
			parts[i] = strconv.FormatUint(f.num, 10)
		} else {
			parts[i] = f.str
		}
	}
	token = strings.Join(parts, ".")

	segments := strings.SplitN(target, ".", len(fields))

	for i, f := range fields {
		// Legacy or truncated target: the remaining fields compare equal.
		if i >= len(segments) {
			return token, 0, true
		}

		if f.isNumeric {
			targetNum, err := strconv.ParseUint(segments[i], 10, 64)
			if err != nil {
				return token, 0, false
			}
			c = cmp.Compare(f.num, targetNum)
		} else {
			c = cmp.Compare(f.str, segments[i])
		}
		if c != 0 {
			return token, c, true
		}
	}

	return token, 0, true
}

// NamespaceIDTokenizer returns a tokenizer by Namespace and ID. Namespace
// cannot contain '.', so it is always the first token segment.
func NamespaceIDTokenizer[T namespaceIDGetter](target string) Tokenizer[T] {
	return func(item T) (string, int) {
		// String fields only, so the target can't be malformed.
		token, c, _ := tokenAndCompare([]tokenField{
			stringField(item.GetNamespace()),
			stringField(item.GetID()),
		}, target)
		return token, c
	}
}

// IDTokenizer returns a tokenizer by ID.
func IDTokenizer[T idGetter](target string) Tokenizer[T] {
	return func(item T) (string, int) {
		// String fields only, so the target can't be malformed.
		token, c, _ := tokenAndCompare([]tokenField{
			stringField(item.GetID()),
		}, target)
		return token, c
	}
}

// CreateIndexAndIDTokenizer returns a tokenizer by CreateIndex and ID. The
// index compares numerically and the ID as a string.
func CreateIndexAndIDTokenizer[T idAndCreateIndexGetter](target string) Tokenizer[T] {
	return func(item T) (string, int) {
		token, c, ok := tokenAndCompare([]tokenField{
			numericField(item.GetCreateIndex()),
			stringField(item.GetID()),
		}, target)

		// No ID or a non-integer index: compare as strings, as before.
		if !ok || !strings.Contains(target, ".") {
			return token, cmp.Compare(token, target)
		}
		return token, c
	}
}

// ModifyIndexAndNamespaceIDTokenizer returns a tokenizer keyed on ModifyIndex,
// then Namespace and ID. ModifyIndex is not unique (several objects may be
// written in one Raft transaction), so it can't identify a unique resume
// position on its own. Namespace and ID break the tie to give a total order
// matching how memdb iterates the non-unique modify_index index: by ModifyIndex,
// then the (Namespace, ID) primary key.
func ModifyIndexAndNamespaceIDTokenizer[T modifyIndexAndNamespaceIDGetter](target string) Tokenizer[T] {
	return func(item T) (string, int) {
		token, c, ok := tokenAndCompare([]tokenField{
			numericField(item.GetModifyIndex()),
			stringField(item.GetNamespace()),
			stringField(item.GetID()),
		}, target)

		// Non-integer index, only from a hand-crafted token: return 0 so paging
		// starts from the beginning, as the old ModifyIndex-only tokenizer did.
		if !ok {
			return token, 0
		}
		return token, c
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
