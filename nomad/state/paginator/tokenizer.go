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

// tokenField is one component of a pagination token. Fields are compared in
// order to produce the token's total ordering, which must match the order the
// underlying memdb index iterates in. A numeric field is compared numerically
// (so 12 sorts before 102, not lexicographically); a string field is compared
// lexicographically.
type tokenField struct {
	str       string
	num       uint64
	isNumeric bool
}

// stringField builds a lexicographically-compared token component.
func stringField(s string) tokenField { return tokenField{str: s} }

// numericField builds a numerically-compared token component.
func numericField(n uint64) tokenField { return tokenField{num: n, isNumeric: true} }

// tokenAndCompare serializes fields into a '.'-joined token and compares that
// token against target field-by-field, in order, returning the token, the
// comparison result (-1, 0, +1), and whether the target was well-formed.
//
// Comparing field-by-field, rather than comparing the joined strings directly,
// is what makes the token's order match memdb's tuple order. memdb's compound
// indexes separate fields with a NUL byte, which sorts before every printable
// character, so the index orders by each field in turn. A direct string compare
// of the joined token uses the '.' separator instead, whose value relative to
// the field characters can invert the order (e.g. namespaces "team" and
// "team-a": memdb sorts "team" first, but "team.id" > "team-a.id" because
// '.' > '-'). Field-by-field comparison avoids that entirely.
//
// Only the final field may contain the '.' separator (e.g. an ID); every
// earlier field (namespace, an index) must not, so the target is split into at
// most len(fields) segments and the last segment absorbs any remaining '.'.
//
// A target with fewer segments than fields (a legacy or truncated token, e.g.
// a bare integer minted before a tiebreaker was added, seen during a rolling
// upgrade) compares only the segments it has and treats the rest as equal,
// degrading to the older, narrower ordering rather than erroring.
//
// A numeric field whose target segment isn't an integer makes the target
// malformed: the comparison stops and ok is false, leaving the fallback to the
// caller, since each tokenizer handles a malformed token differently.
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
		// The target ran out of segments; everything so far matched, so treat
		// the remaining fields as equal (legacy/truncated token).
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

// NamespaceIDTokenizer returns a tokenizer by Namespace and ID. The token is
// `namespace.id`; comparison is field-by-field so it matches the memdb
// (Namespace, ID) iteration order (Namespace cannot contain '.', so it is
// always the first segment).
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
// token is `createIndex.id`; the index is compared numerically and the ID
// lexicographically.
func CreateIndexAndIDTokenizer[T idAndCreateIndexGetter](target string) Tokenizer[T] {
	return func(item T) (string, int) {
		token, c, ok := tokenAndCompare([]tokenField{
			numericField(item.GetCreateIndex()),
			stringField(item.GetID()),
		}, target)

		// A target missing either part, or with a non-integer index, is
		// compared directly as a string.
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

		// A target whose first segment isn't an integer is malformed -- only
		// reachable from a hand-crafted token, since real next_tokens are empty
		// or start with an index -- so degrade to first-page behavior:
		// returning 0 ("matched") makes the paginator skip nothing and start
		// from the beginning, matching the previous ModifyIndex-only tokenizer.
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
