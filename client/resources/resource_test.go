package resources

import (
	"fmt"
	"github.com/hashicorp/hcl"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestResource_Range(t *testing.T) {
	type testCase struct {
		name     string
		_type    string
		config   interface{}
		lower    int64
		upper    int64
		errorMsg string
	}

	testCases := []testCase{
		{
			name:     "valid-between",
			_type:    "range",
			config:   5,
			lower:    0,
			upper:    10,
			errorMsg: "",
		},
		{
			name:     "valid-matches-lower-bound",
			_type:    "range",
			config:   5,
			lower:    5,
			upper:    10,
			errorMsg: "",
		},
		{
			name:     "valid-matches-upper-bound",
			_type:    "range",
			config:   10,
			lower:    5,
			upper:    10,
			errorMsg: "",
		},
		{
			name:     "invalid-less-than-lower-bound",
			_type:    "range",
			config:   4,
			lower:    5,
			upper:    10,
			errorMsg: "cannot be less than lower bound",
		},
		{
			name:     "invalid-greater-than-upper-bound",
			_type:    "range",
			config:   4,
			lower:    5,
			upper:    10,
			errorMsg: "cannot be greater than upper bound",
		},
		{
			name:     "invalid-typecast-error",
			_type:    "range",
			config:   4,
			lower:    5,
			upper:    10,
			errorMsg: "cannot be cast to int64",
		},
	}

	rangeTmpl := `
resource "%s" {
  range {
    lower = %d
    upper   = %d
  }
}
`
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resourceHCL := fmt.Sprintf(rangeTmpl, tc.name, tc.lower, tc.upper)
			resource := &Resource{}
			err := hcl.Decode(resource, resourceHCL)
			require.NoError(t, err, "cannot parse range hcl")

			require.NotNil(t, resource)
			require.NotNil(t, resource.Range)
			require.Equal(t, tc.lower, resource.Range.Lower)
			require.Equal(t, tc.upper, resource.Range.Upper)

			err = resource.Range.Validate(tc.config)

			if tc.errorMsg == "" {
				require.NoError(t, err)
			} else {
				require.NotNil(t, err)
				require.ErrorContains(t, err, tc.errorMsg)
			}
		})
	}
}
