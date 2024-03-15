// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package config

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/shoenig/test/must"
)

func TestUsersConfigFromAgent(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		name   string
		config *config.UsersConfig
		exp    *UsersConfig
	}{
		{
			name:   "from default",
			config: config.DefaultUsersConfig(),
			exp: &UsersConfig{
				MinDynamicUser: 80_000,
				MaxDynamicUser: 89_999,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := UsersConfigFromAgent(tc.config)
			must.Eq(t, tc.exp, got)
		})
	}
}

func TestUsersConfig_Copy(t *testing.T) {
	ci.Parallel(t)

	orig := &UsersConfig{
		MinDynamicUser: 70100,
		MaxDynamicUser: 70200,
	}

	configCopy := orig.Copy()
	must.Eq(t, orig, configCopy)

	// modify copy and make sure original does not change
	configCopy.MinDynamicUser = 100
	configCopy.MaxDynamicUser = 200

	must.Eq(t, &UsersConfig{
		MinDynamicUser: 70100,
		MaxDynamicUser: 70200,
	}, orig)
}
