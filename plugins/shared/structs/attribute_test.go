package structs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAttribute_ParseAndValidate(t *testing.T) {
	cases := []struct {
		Input    string
		Expected *Attribute
	}{
		{
			Input: "true",
			Expected: &Attribute{
				Bool: true,
			},
		},
		{
			Input: "false",
			Expected: &Attribute{
				Bool: false,
			},
		},
		{
			Input: "100",
			Expected: &Attribute{
				Int: 100,
			},
		},
		{
			Input: "-100",
			Expected: &Attribute{
				Int: -100,
			},
		},
		{
			Input: "-1.0",
			Expected: &Attribute{
				Float: -1.0,
			},
		},
		{
			Input: "-100.25",
			Expected: &Attribute{
				Float: -100.25,
			},
		},
		{
			Input: "1.01",
			Expected: &Attribute{
				Float: 1.01,
			},
		},
		{
			Input: "100.25",
			Expected: &Attribute{
				Float: 100.25,
			},
		},
		{
			Input: "foobar",
			Expected: &Attribute{
				String: "foobar",
			},
		},
		{
			Input: "foo123bar",
			Expected: &Attribute{
				String: "foo123bar",
			},
		},
		{
			Input: "100MB",
			Expected: &Attribute{
				Int:  100,
				Unit: "MB",
			},
		},
		{
			Input: "-100MHz",
			Expected: &Attribute{
				Int:  -100,
				Unit: "MHz",
			},
		},
		{
			Input: "-1.0MB/s",
			Expected: &Attribute{
				Float: -1.0,
				Unit:  "MB/s",
			},
		},
		{
			Input: "-100.25GiB/s",
			Expected: &Attribute{
				Float: -100.25,
				Unit:  "GiB/s",
			},
		},
		{
			Input: "1.01TB",
			Expected: &Attribute{
				Float: 1.01,
				Unit:  "TB",
			},
		},
		{
			Input: "100.25mW",
			Expected: &Attribute{
				Float: 100.25,
				Unit:  "mW",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.Input, func(t *testing.T) {
			a := ParseAttribute(c.Input)
			require.Equal(t, c.Expected, a)
			require.NoError(t, a.Validate())
		})
	}
}
