package structs

import (
	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
	"testing"
)

func TestCNIConfig_Equal(t *testing.T) {
	ci.Parallel(t)

	must.Equal[*CNIArgs](t, nil, nil)
	must.NotEqual[*CNIArgs](t, nil, new(CNIArgs))
	must.NotEqual[*CNIArgs](t, nil, &CNIArgs{Args: map[string]string{"first": "second"}})

	must.StructEqual(t, &CNIArgs{
		Args: map[string]string{
			"arg":     "example_1",
			"new_arg": "example_2",
		},
	}, []must.Tweak[*CNIArgs]{{
		Field: "Args",
		Apply: func(c *CNIArgs) { c.Args = map[string]string{"different": "arg"} },
	}})
}
