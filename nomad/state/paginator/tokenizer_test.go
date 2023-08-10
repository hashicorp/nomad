// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package paginator

import (
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/stretchr/testify/require"
)

func TestStructsTokenizer(t *testing.T) {
	ci.Parallel(t)

	j := mock.Job()

	cases := []struct {
		name     string
		opts     StructsTokenizerOptions
		expected string
	}{
		{
			name: "ID",
			opts: StructsTokenizerOptions{
				WithID: true,
			},
			expected: fmt.Sprintf("%v", j.ID),
		},
		{
			name: "Namespace.ID",
			opts: StructsTokenizerOptions{
				WithNamespace: true,
				WithID:        true,
			},
			expected: fmt.Sprintf("%v.%v", j.Namespace, j.ID),
		},
		{
			name: "CreateIndex.Namespace.ID",
			opts: StructsTokenizerOptions{
				WithCreateIndex: true,
				WithNamespace:   true,
				WithID:          true,
			},
			expected: fmt.Sprintf("%v.%v.%v", j.CreateIndex, j.Namespace, j.ID),
		},
		{
			name: "CreateIndex.ID",
			opts: StructsTokenizerOptions{
				WithCreateIndex: true,
				WithID:          true,
			},
			expected: fmt.Sprintf("%v.%v", j.CreateIndex, j.ID),
		},
		{
			name: "CreateIndex.Namespace",
			opts: StructsTokenizerOptions{
				WithCreateIndex: true,
				WithNamespace:   true,
			},
			expected: fmt.Sprintf("%v.%v", j.CreateIndex, j.Namespace),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tokenizer := StructsTokenizer{opts: tc.opts}
			require.Equal(t, tc.expected, tokenizer.GetToken(j))
		})
	}
}
