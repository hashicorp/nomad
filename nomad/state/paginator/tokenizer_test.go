// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package paginator

import (
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func TestTokenizer(t *testing.T) {
	ci.Parallel(t)

	j := mock.Job()

	cases := []struct {
		name      string
		tokenizer Tokenizer[*structs.Job]
		expected  string
	}{
		{
			name:      "ID",
			tokenizer: IDTokenizer[*structs.Job](""),
			expected:  fmt.Sprintf("%v", j.ID),
		},
		{
			name:      "Namespace.ID",
			tokenizer: NamespaceIDTokenizer[*structs.Job](""),
			expected:  fmt.Sprintf("%v.%v", j.Namespace, j.ID),
		},
		{
			name:      "CreateIndex.ID",
			tokenizer: CreateIndexAndIDTokenizer[*structs.Job](""),
			expected:  fmt.Sprintf("%v.%v", j.CreateIndex, j.ID),
		},
		{
			name:      "ModifyIndex.Namespace.ID",
			tokenizer: ModifyIndexAndNamespaceIDTokenizer[*structs.Job](""),
			expected:  fmt.Sprintf("%d.%v.%v", j.ModifyIndex, j.Namespace, j.ID),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			token, _ := tc.tokenizer(j)
			must.Eq(t, tc.expected, token)
		})
	}
}

func TestCreateIndexAndIDTokenizer(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		name          string
		obj           *mockCreateIndexObject
		target        string
		expectedToken string
		expectedCmp   int
	}{
		{
			name:          "common index (less)",
			obj:           newMockCreateIndexObject(12, "aaa-bbb-ccc"),
			target:        "12.bbb-ccc-ddd",
			expectedToken: "12.aaa-bbb-ccc",
			expectedCmp:   -1,
		},
		{
			name:          "common index (greater)",
			obj:           newMockCreateIndexObject(12, "bbb-ccc-ddd"),
			target:        "12.aaa-bbb-ccc",
			expectedToken: "12.bbb-ccc-ddd",
			expectedCmp:   1,
		},
		{
			name:          "common index (equal)",
			obj:           newMockCreateIndexObject(12, "bbb-ccc-ddd"),
			target:        "12.bbb-ccc-ddd",
			expectedToken: "12.bbb-ccc-ddd",
			expectedCmp:   0,
		},
		{
			name:          "less index",
			obj:           newMockCreateIndexObject(12, "aaa-bbb-ccc"),
			target:        "89.aaa-bbb-ccc",
			expectedToken: "12.aaa-bbb-ccc",
			expectedCmp:   -1,
		},
		{
			name:          "greater index",
			obj:           newMockCreateIndexObject(89, "aaa-bbb-ccc"),
			target:        "12.aaa-bbb-ccc",
			expectedToken: "89.aaa-bbb-ccc",
			expectedCmp:   1,
		},
		{
			name:          "common index start (less)",
			obj:           newMockCreateIndexObject(12, "aaa-bbb-ccc"),
			target:        "102.aaa-bbb-ccc",
			expectedToken: "12.aaa-bbb-ccc",
			expectedCmp:   -1,
		},
		{
			name:          "common index start (greater)",
			obj:           newMockCreateIndexObject(102, "aaa-bbb-ccc"),
			target:        "12.aaa-bbb-ccc",
			expectedToken: "102.aaa-bbb-ccc",
			expectedCmp:   1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fn := CreateIndexAndIDTokenizer[*mockCreateIndexObject](tc.target)
			actualToken, actualCmp := fn(tc.obj)
			must.Eq(t, tc.expectedToken, actualToken)
			must.Eq(t, tc.expectedCmp, actualCmp)
		})
	}
}

func newMockCreateIndexObject(createIndex uint64, id string) *mockCreateIndexObject {
	return &mockCreateIndexObject{
		createIndex: createIndex,
		id:          id,
	}
}

type mockCreateIndexObject struct {
	createIndex uint64
	id          string
}

func (m *mockCreateIndexObject) GetCreateIndex() uint64 {
	return m.createIndex
}

func (m *mockCreateIndexObject) GetID() string {
	return m.id
}

func TestModifyIndexAndNamespaceIDTokenizer(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		name          string
		obj           *mockModifyIndexObject
		target        string
		expectedToken string
		expectedCmp   int
	}{
		{
			name:          "less index",
			obj:           newMockModifyIndexObject(12, "default", "aaa"),
			target:        "89.default.aaa",
			expectedToken: "12.default.aaa",
			expectedCmp:   -1,
		},
		{
			name:          "greater index",
			obj:           newMockModifyIndexObject(89, "default", "aaa"),
			target:        "12.default.aaa",
			expectedToken: "89.default.aaa",
			expectedCmp:   1,
		},
		{
			// index is compared numerically, not lexically: 12 < 102
			name:          "numeric index (less)",
			obj:           newMockModifyIndexObject(12, "default", "aaa"),
			target:        "102.default.aaa",
			expectedToken: "12.default.aaa",
			expectedCmp:   -1,
		},
		{
			name:          "equal index, less namespace",
			obj:           newMockModifyIndexObject(12, "aaa", "x"),
			target:        "12.bbb.x",
			expectedToken: "12.aaa.x",
			expectedCmp:   -1,
		},
		{
			name:          "equal index, greater namespace",
			obj:           newMockModifyIndexObject(12, "bbb", "x"),
			target:        "12.aaa.x",
			expectedToken: "12.bbb.x",
			expectedCmp:   1,
		},
		{
			// namespace compared field-by-field, so "team" sorts before
			// "team-a" (matching memdb's null-separated key order), unlike a
			// whole-string compare where '.' > '-' would reverse them.
			name:          "dash namespace ordering",
			obj:           newMockModifyIndexObject(12, "team", "z"),
			target:        "12.team-a.a",
			expectedToken: "12.team.z",
			expectedCmp:   -1,
		},
		{
			name:          "equal index and namespace, less id",
			obj:           newMockModifyIndexObject(12, "team", "aaa"),
			target:        "12.team.bbb",
			expectedToken: "12.team.aaa",
			expectedCmp:   -1,
		},
		{
			name:          "equal index, namespace, and id",
			obj:           newMockModifyIndexObject(12, "team", "aaa"),
			target:        "12.team.aaa",
			expectedToken: "12.team.aaa",
			expectedCmp:   0,
		},
		{
			// id may contain '.'; it's the remainder after the first two splits.
			name:          "id with dots",
			obj:           newMockModifyIndexObject(12, "default", "a.b.c"),
			target:        "12.default.a.b.c",
			expectedToken: "12.default.a.b.c",
			expectedCmp:   0,
		},
		{
			// legacy bare-integer token (rolling upgrade): index-only compare
			name:          "legacy bare-integer target (equal)",
			obj:           newMockModifyIndexObject(12, "default", "aaa"),
			target:        "12",
			expectedToken: "12.default.aaa",
			expectedCmp:   0,
		},
		{
			name:          "legacy bare-integer target (less)",
			obj:           newMockModifyIndexObject(12, "default", "aaa"),
			target:        "13",
			expectedToken: "12.default.aaa",
			expectedCmp:   -1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fn := ModifyIndexAndNamespaceIDTokenizer[*mockModifyIndexObject](tc.target)
			actualToken, actualCmp := fn(tc.obj)
			must.Eq(t, tc.expectedToken, actualToken)
			must.Eq(t, tc.expectedCmp, actualCmp)
		})
	}
}

func newMockModifyIndexObject(modifyIndex uint64, namespace, id string) *mockModifyIndexObject {
	return &mockModifyIndexObject{
		modifyIndex: modifyIndex,
		namespace:   namespace,
		id:          id,
	}
}

type mockModifyIndexObject struct {
	modifyIndex uint64
	namespace   string
	id          string
}

func (m *mockModifyIndexObject) GetModifyIndex() uint64 { return m.modifyIndex }
func (m *mockModifyIndexObject) GetNamespace() string   { return m.namespace }
func (m *mockModifyIndexObject) GetID() string          { return m.id }
