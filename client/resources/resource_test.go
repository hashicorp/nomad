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

	rangeTmpl := `
resource "%s" {
	config {
		type  = "range"
   		lower = %d
   		upper = %d
 	}
}
	`

	type testCase struct {
		name      string
		_type     string
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
			_type:     "range",
			value:     5,
			lower:     0,
			upper:     10,
			parseErr:  "illegal char",
			cfgErrMsg: "",
			errMsg:    "",
			tmpl: `
resource "%s" {
	config {
		type  = "range"
   		upper = %d
 	}
}
	`,
		},
		{
			name:      "invalid-config-lower-bound",
			_type:     "range",
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
			_type:     "range",
			value:     5,
			lower:     0,
			upper:     10,
			parseErr:  "illegal char",
			cfgErrMsg: "",
			errMsg:    "",
			tmpl: `
resource "%s" {
	config {
		type  = "range"
   		lower = %d
 	}
}
	`,
		},
		{
			name:      "invalid-config-upper-bound",
			_type:     "range",
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
			_type:     "range",
			value:     5,
			lower:     10,
			upper:     5,
			cfgErrMsg: "which is greater than upper bound",
			errMsg:    "",
			tmpl:      rangeTmpl,
		},
		{
			name:      "valid-between",
			_type:     "range",
			value:     5,
			lower:     0,
			upper:     10,
			cfgErrMsg: "",
			errMsg:    "",
			tmpl:      rangeTmpl,
		},
		{
			name:      "valid-matches-lower-bound",
			_type:     "range",
			value:     5,
			lower:     5,
			upper:     10,
			cfgErrMsg: "",
			errMsg:    "",
			tmpl:      rangeTmpl,
		},
		{
			name:      "valid-matches-upper-bound",
			_type:     "range",
			value:     10,
			lower:     5,
			upper:     10,
			cfgErrMsg: "",
			errMsg:    "",
			tmpl:      rangeTmpl,
		},
		{
			name:      "invalid-less-than-lower-bound",
			_type:     "range",
			value:     4,
			lower:     5,
			upper:     10,
			cfgErrMsg: "",
			errMsg:    "cannot be less than lower bound",
			tmpl:      rangeTmpl,
		},
		{
			name:      "invalid-greater-than-upper-bound",
			_type:     "range",
			value:     11,
			lower:     5,
			upper:     10,
			cfgErrMsg: "",
			errMsg:    "cannot be greater than upper bound",
			tmpl:      rangeTmpl,
		},
		{
			name:      "invalid-typecast-error",
			_type:     "range",
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
			cfg := &testConfig{}
			err := hcl.Decode(cfg, resourceHCL)

			if tc.parseErr != "" {
				require.ErrorContains(t, err, tc.parseErr)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, cfg)
			require.Len(t, cfg.Resources, 1)
			resourceCfg := cfg.Resources[0]
			require.Equal(t, tc.name, resourceCfg.Name)
			require.NotNil(t, resourceCfg.Config)
			require.Equal(t, "range", resourceCfg.Config["type"])

			err = resourceCfg.Validate()

			if tc.cfgErrMsg == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.cfgErrMsg)
				return
			}

			validator, err := NewValidator(resourceCfg)
			require.NoError(t, err)

			err = validator.Validate(tc.value)

			if tc.errMsg == "" {
				require.NoError(t, err)
				err = validator.Validate(tc.value)
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.ErrorContains(t, err, tc.errMsg)
			}
		})
	}
}
