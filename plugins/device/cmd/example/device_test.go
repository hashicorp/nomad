//Copyright IBM Corp. 2015, 2026
//SPDX-License-Identifier: BUSL-1.1

package example

import (
	"testing"

	"github.com/shoenig/test/must"
)

func TestSanitizeDir(t *testing.T) {
	cases := []struct {
		name        string
		path        string
		expectedErr string
	}{
		{
			name:        "relative segment",
			path:        "../home/fine",
			expectedErr: ".. is not allowed in dir when running in dynamic mode",
		},
		{
			name:        "disallowed segment",
			path:        "C:\\WINDOWS\\Teams",
			expectedErr: "WINDOWS is not allowed in dir when running in dynamic mode",
		},
		{
			name:        "disallowed segment",
			path:        "./home",
			expectedErr: ". is not allowed in dir when running in dynamic mode",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := sanitizeDir(tc.path)
			if tc.expectedErr != "" {
				must.NotNil(t, err)
				must.Eq(t, tc.expectedErr, err.Error())
			}
		})
	}
}
