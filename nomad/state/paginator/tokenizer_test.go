// Copyright (c) HashiCorp, Inc.
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
			name:      "ModifyIndex",
			tokenizer: ModifyIndexTokenizer[*structs.Job](""),
			expected:  fmt.Sprintf("%d", j.ModifyIndex),
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
