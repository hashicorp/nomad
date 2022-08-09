package resources

import (
	"testing"

	"fmt"
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
		formatFn  func(testCase) string
	}

	defaultFormatFn := func(tc testCase) string {
		return fmt.Sprintf(tc.tmpl, tc.name, tc.lower, tc.upper)
	}

	testCases := []testCase{
		{
			name:      "invalid-config-no-lower-bound",
			value:     5,
			lower:     nil,
			upper:     10,
			parseErr:  "The argument \"lower\" is required",
			cfgErrMsg: "",
			errMsg:    "",
			tmpl: `
resource "%s" {
	range {
   		upper = %d
 	}
}
	`,
			formatFn: func(tc testCase) string {
				return fmt.Sprintf(tc.tmpl, tc.name, tc.upper)
			},
		},
		{
			name:      "invalid-config-lower-bound",
			value:     5,
			lower:     false,
			upper:     10,
			parseErr:  "invalid expression token",
			cfgErrMsg: "",
			errMsg:    "",
			tmpl:      rangeTmpl,
			formatFn:  defaultFormatFn,
		},
		{
			name:      "invalid-config-no-upper-bound",
			value:     5,
			lower:     0,
			upper:     nil,
			parseErr:  "The argument \"upper\" is required",
			cfgErrMsg: "",
			errMsg:    "",
			tmpl: `
resource "%s" {
	range {
   		lower = %d
 	}
}
	`,
			formatFn: func(tc testCase) string {
				return fmt.Sprintf(tc.tmpl, tc.name, tc.lower)
			},
		},
		{
			name:      "invalid-config-upper-bound",
			value:     5,
			lower:     0,
			upper:     "foo",
			parseErr:  "Invalid expression",
			cfgErrMsg: "",
			errMsg:    "",
			tmpl:      rangeTmpl,
			formatFn:  defaultFormatFn,
		},
		{
			name:      "invalid-config-lower-greater-than-upper",
			value:     5,
			lower:     10,
			upper:     5,
			parseErr:  "",
			cfgErrMsg: "greater than upper bound",
			errMsg:    "",
			tmpl:      rangeTmpl,
			formatFn:  defaultFormatFn,
		},
		{
			name:      "valid-between",
			value:     5,
			lower:     0,
			upper:     10,
			cfgErrMsg: "",
			errMsg:    "",
			tmpl:      rangeTmpl,
			formatFn:  defaultFormatFn,
		},
		{
			name:      "valid-matches-lower-bound",
			value:     5,
			lower:     5,
			upper:     10,
			cfgErrMsg: "",
			errMsg:    "",
			tmpl:      rangeTmpl,
			formatFn:  defaultFormatFn,
		},
		{
			name:      "valid-matches-upper-bound",
			value:     10,
			lower:     5,
			upper:     10,
			cfgErrMsg: "",
			errMsg:    "",
			tmpl:      rangeTmpl,
			formatFn:  defaultFormatFn,
		},
		{
			name:      "invalid-less-than-lower-bound",
			value:     4,
			lower:     5,
			upper:     10,
			cfgErrMsg: "",
			errMsg:    "cannot be less than lower bound",
			tmpl:      rangeTmpl,
			formatFn:  defaultFormatFn,
		},
		{
			name:      "invalid-greater-than-upper-bound",
			value:     11,
			lower:     5,
			upper:     10,
			cfgErrMsg: "",
			errMsg:    "cannot be greater than upper bound",
			tmpl:      rangeTmpl,
			formatFn:  defaultFormatFn,
		},
		{
			name:      "invalid-typecast-error",
			value:     "foo",
			lower:     5,
			upper:     10,
			cfgErrMsg: "",
			errMsg:    "cannot be cast to int",
			tmpl:      rangeTmpl,
			formatFn:  defaultFormatFn,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resourceHCL := tc.formatFn(tc)
			resource, diags := Parse(resourceHCL, tc.name)

			if tc.parseErr != "" {
				require.NotNil(t, diags)
				require.Contains(t, diags.Error(), tc.parseErr)
				return
			}

			require.NoError(t, diags)
			require.NotNil(t, resource)
			require.Equal(t, tc.name, resource.Name)
			require.NotNil(t, resource.Range)

			err := resource.ValidateConfig()

			if tc.cfgErrMsg == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.cfgErrMsg)
				return
			}

			err = resource.Range.Validate(tc.value)

			if tc.errMsg == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.ErrorContains(t, err, tc.errMsg)
			}
		})
	}
}

func TestResource_Set(t *testing.T) {
	setTmpl := `
resource "%s" {
	set {
   		members = [%s]
 	}
}
	`

	type testCase struct {
		name      string
		value     interface{}
		members   string
		parseErr  string
		cfgErrMsg string
		errMsg    string
		tmpl      string
		formatFn  func(testCase) string
	}

	defaultFormatFn := func(tc testCase) string {
		return fmt.Sprintf(tc.tmpl, tc.name, tc.members)
	}

	testCases := []testCase{
		{
			name:      "invalid-config-no-members",
			members:   "",
			value:     "1234",
			parseErr:  "The argument \"members\" is required",
			cfgErrMsg: "",
			errMsg:    "",
			tmpl: `
resource "%s" {
	set {
 	}
}
	`,
			formatFn: func(tc testCase) string {
				return fmt.Sprintf(tc.tmpl, tc.name)
			},
		},
		{
			name:      "invalid-config-empty-members",
			members:   "",
			value:     "1234",
			parseErr:  "",
			cfgErrMsg: "has no members",
			errMsg:    "",
			tmpl:      setTmpl,
			formatFn:  defaultFormatFn,
		},
		{
			name:      "invalid-config-duplicate",
			members:   "\"1234\",\"1234\"",
			value:     "1234",
			parseErr:  "",
			cfgErrMsg: "more than once",
			errMsg:    "",
			tmpl:      setTmpl,
			formatFn:  defaultFormatFn,
		},
		{
			name:      "valid",
			members:   "\"1234\",\"abcd\"",
			value:     "1234",
			parseErr:  "",
			cfgErrMsg: "",
			errMsg:    "",
			tmpl:      setTmpl,
			formatFn:  defaultFormatFn,
		},
		{
			name:      "invalid-does-not-exist",
			members:   "\"1234\",\"abcd\"",
			value:     "4321",
			parseErr:  "",
			cfgErrMsg: "",
			errMsg:    "not a member",
			tmpl:      setTmpl,
			formatFn:  defaultFormatFn,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resourceHCL := tc.formatFn(tc)
			resource, diags := Parse(resourceHCL, tc.name)

			if tc.parseErr != "" {
				require.NotNil(t, diags)
				require.Contains(t, diags.Error(), tc.parseErr)
				return
			}

			require.NoError(t, diags)
			require.NotNil(t, resource)
			require.Equal(t, tc.name, resource.Name)
			require.NotNil(t, resource.Set)

			err := resource.ValidateConfig()

			if tc.cfgErrMsg == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.cfgErrMsg)
				return
			}

			err = resource.Set.Validate(tc.value)

			if tc.errMsg == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.ErrorContains(t, err, tc.errMsg)
			}
		})
	}
}
