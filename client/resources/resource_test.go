package resources

import (
	"testing"

	"fmt"
	"github.com/hashicorp/hcl"
	"github.com/stretchr/testify/require"
)

func TestResource_Range(t *testing.T) {
	rangeTmpl := `
resource "%s" {
	range {
   		lower = %d
   		upper = %d
 	}
}
	`

	type testCase struct {
		name      string
		value     interface{}
		lower     interface{}
		upper     interface{}
		parseErr  string
		cfgErrMsg string
		errMsg    string
		tmpl      string
	}

	testCases := []testCase{
		{
			name:      "invalid-config-no-lower-bound",
			value:     5,
			lower:     0,
			upper:     10,
			parseErr:  "illegal char",
			cfgErrMsg: "",
			errMsg:    "",
			tmpl: `
resource "%s" {
	range {
   		upper = %d
 	}
}
	`,
		},
		{
			name:      "invalid-config-lower-bound",
			value:     5,
			lower:     false,
			upper:     10,
			parseErr:  "illegal char",
			cfgErrMsg: "",
			errMsg:    "",
			tmpl:      rangeTmpl,
		},
		{
			name:      "invalid-config-no-upper-bound",
			value:     5,
			lower:     0,
			upper:     10,
			parseErr:  "illegal char",
			cfgErrMsg: "",
			errMsg:    "",
			tmpl: `
resource "%s" {
	range {
   		lower = %d
 	}
}
	`,
		},
		{
			name:      "invalid-config-upper-bound",
			value:     5,
			lower:     0,
			upper:     "foo",
			parseErr:  "illegal char",
			cfgErrMsg: "",
			errMsg:    "",
			tmpl:      rangeTmpl,
		},
		{
			name:      "invalid-config-lower-greater-than-upper",
			value:     5,
			lower:     10,
			upper:     5,
			cfgErrMsg: "greater than upper bound",
			errMsg:    "",
			tmpl:      rangeTmpl,
		},
		{
			name:      "valid-between",
			value:     5,
			lower:     0,
			upper:     10,
			cfgErrMsg: "",
			errMsg:    "",
			tmpl:      rangeTmpl,
		},
		{
			name:      "valid-matches-lower-bound",
			value:     5,
			lower:     5,
			upper:     10,
			cfgErrMsg: "",
			errMsg:    "",
			tmpl:      rangeTmpl,
		},
		{
			name:      "valid-matches-upper-bound",
			value:     10,
			lower:     5,
			upper:     10,
			cfgErrMsg: "",
			errMsg:    "",
			tmpl:      rangeTmpl,
		},
		{
			name:      "invalid-less-than-lower-bound",
			value:     4,
			lower:     5,
			upper:     10,
			cfgErrMsg: "",
			errMsg:    "cannot be less than lower bound",
			tmpl:      rangeTmpl,
		},
		{
			name:      "invalid-greater-than-upper-bound",
			value:     11,
			lower:     5,
			upper:     10,
			cfgErrMsg: "",
			errMsg:    "cannot be greater than upper bound",
			tmpl:      rangeTmpl,
		},
		{
			name:      "invalid-typecast-error",
			value:     "foo",
			lower:     5,
			upper:     10,
			cfgErrMsg: "",
			errMsg:    "cannot be cast to int",
			tmpl:      rangeTmpl,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resourceHCL := fmt.Sprintf(tc.tmpl, tc.name, tc.lower, tc.upper)
			resource := &Resource{}
			err := hcl.Decode(resource, resourceHCL)

			if tc.parseErr != "" {
				require.ErrorContains(t, err, tc.parseErr)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, resource)
			require.Equal(t, tc.name, resource.Name)
			require.NotNil(t, resource.Range)

			err = resource.ValidateConfig()

			if tc.cfgErrMsg == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.cfgErrMsg)
				return
			}

			err = resource.Range.Validate(tc.value.(int))

			if tc.errMsg == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.ErrorContains(t, err, tc.errMsg)
			}
		})
	}
}
