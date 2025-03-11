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
