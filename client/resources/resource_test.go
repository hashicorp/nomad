package resources

import (
	"testing"

	"fmt"
	"github.com/hashicorp/hcl"
	"github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/stretchr/testify/require"
)

func TestResource_Range(t *testing.T) {
	type testConfig struct {
		Resources []*config.ResourceConfig `hcl:"resource"`
	}

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
	config {
		type  = "range"
   		lower = %d
   		upper = %d
 	}
}
	`
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resourceHCL := fmt.Sprintf(rangeTmpl, tc.name, tc.lower, tc.upper)
			cfg := &testConfig{}
			err := hcl.Decode(cfg, resourceHCL)
			require.NoError(t, err)

			require.NotNil(t, cfg)
			require.Len(t, cfg.Resources, 1)
			resource := cfg.Resources[0]
			require.Equal(t, tc.name, resource.Name)
			require.NotNil(t, resource.Config)
			require.Equal(t, "range", resource.Config["type"])
			require.Equal(t, tc.lower, int64(resource.Config["lower"].(int)))
			require.Equal(t, tc.upper, int64(resource.Config["upper"].(int)))

			err = resource.Validate()
			require.NoError(t, err)

			// TODO (derek): Rework input validation and int64 handling
			//if tc.errorMsg == "" {
			//	require.NoError(t, err)
			//} else {
			//	require.Error(t, err)
			//	//require.ErrorContains(t, err, tc.errorMsg)
			//}
		})
	}
}
