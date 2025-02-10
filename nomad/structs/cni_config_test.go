// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
)

func TestCNIConfig_Equal(t *testing.T) {
	ci.Parallel(t)

	must.Equal[*CNIConfig](t, nil, nil)
	must.NotEqual[*CNIConfig](t, nil, new(CNIConfig))
	must.NotEqual[*CNIConfig](t, nil, &CNIConfig{Args: map[string]string{"first": "second"}})

	must.StructEqual(t, &CNIConfig{
		Args: map[string]string{
			"arg":     "example_1",
			"new_arg": "example_2",
		},
	}, []must.Tweak[*CNIConfig]{{
		Field: "Args",
		Apply: func(c *CNIConfig) { c.Args = map[string]string{"different": "arg"} },
	}})
}
