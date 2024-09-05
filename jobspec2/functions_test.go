// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package jobspec2

import (
	"testing"

	"github.com/shoenig/test"
)

func TestFunction_fileEscape(t *testing.T) {

	testCases := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "$ needs escape",
			input:  ` ${foo}`,
			expect: ` $${foo}`,
		},
		{
			name:   "$ already escaped",
			input:  ` $${foo}`,
			expect: ` $${foo}`,
		},
		{
			name:   "$ without bracket no escape",
			input:  ` $foo`,
			expect: ` $foo`,
		},
		{
			name:   "no escaped characters",
			input:  ` foo`,
			expect: ` foo`,
		},

		{
			name:   "% needs escape",
			input:  ` %{foo}`,
			expect: ` %%{foo}`,
		},
		{
			name:   "% already escaped",
			input:  ` %%{foo}`,
			expect: ` %%{foo}`,
		},
		{
			name:   "% without bracket no escape",
			input:  ` %foo`,
			expect: ` %foo`,
		},

		{
			name:   "bracket without $ or % no escape",
			input:  ` {foo}`,
			expect: ` {foo}`,
		},
	}

	for _, tc := range testCases {
		test.Eq(t, tc.expect, escape(tc.input),
			test.Sprintf("%v: %v", tc.input, tc.name))
	}
}
