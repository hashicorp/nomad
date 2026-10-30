// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package config

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
)

func TestSELinuxConfig_Copy(t *testing.T) {
	ci.Parallel(t)

	a := &SELinuxConfig{LabelTaskSockets: new(true)}
	b := a.Copy()
	must.Eq(t, a, b)

	*a.LabelTaskSockets = false
	must.NotEq(t, a, b)
}

func TestSELinuxConfig_Merge(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		name   string
		source *SELinuxConfig
		other  *SELinuxConfig
		exp    *SELinuxConfig
	}{
		{
			name:   "both nil",
			source: nil,
			other:  nil,
			exp:    nil,
		},
		{
			name:   "source nil",
			source: nil,
			other:  &SELinuxConfig{LabelTaskSockets: new(true)},
			exp:    &SELinuxConfig{LabelTaskSockets: new(true)},
		},
		{
			name:   "other nil",
			source: &SELinuxConfig{LabelTaskSockets: new(true)},
			other:  nil,
			exp:    &SELinuxConfig{LabelTaskSockets: new(true)},
		},
		{
			name:   "other unset",
			source: &SELinuxConfig{LabelTaskSockets: new(true)},
			other:  &SELinuxConfig{},
			exp:    &SELinuxConfig{LabelTaskSockets: new(true)},
		},
		{
			name:   "other wins",
			source: &SELinuxConfig{LabelTaskSockets: new(true)},
			other:  &SELinuxConfig{LabelTaskSockets: new(false)},
			exp:    &SELinuxConfig{LabelTaskSockets: new(false)},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			must.Eq(t, tc.exp, tc.source.Merge(tc.other))
		})
	}
}
