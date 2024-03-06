// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package config

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/shoenig/test/must"
)

func TestUsersConfig_Copy(t *testing.T) {
	ci.Parallel(t)

	a := DefaultUsersConfig()
	b := a.Copy()
	must.Equal(t, a, b)
	must.Equal(t, b, a)

	a.MaxDynamicUser = pointer.Of(1000)
	must.NotEqual(t, a, b)
	must.NotEqual(t, b, a)
}

func TestUsersConfig_Merge(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		name   string
		source *UsersConfig
		other  *UsersConfig
		exp    *UsersConfig
	}{
		{
			name: "merge all fields",
			source: &UsersConfig{
				MinDynamicUser: pointer.Of(100),
				MaxDynamicUser: pointer.Of(200),
			},
			other: &UsersConfig{
				MinDynamicUser: pointer.Of(3000),
				MaxDynamicUser: pointer.Of(4000),
			},
			exp: &UsersConfig{
				MinDynamicUser: pointer.Of(3000),
				MaxDynamicUser: pointer.Of(4000),
			},
		},
		{
			name:   "null source",
			source: nil,
			other: &UsersConfig{
				MinDynamicUser: pointer.Of(100),
				MaxDynamicUser: pointer.Of(200),
			},
			exp: &UsersConfig{
				MinDynamicUser: pointer.Of(100),
				MaxDynamicUser: pointer.Of(200),
			},
		},
		{
			name:  "null other",
			other: nil,
			source: &UsersConfig{
				MinDynamicUser: pointer.Of(100),
				MaxDynamicUser: pointer.Of(200),
			},
			exp: &UsersConfig{
				MinDynamicUser: pointer.Of(100),
				MaxDynamicUser: pointer.Of(200),
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.source.Merge(tc.other)
			must.Equal(t, tc.exp, got)
		})
	}
}

func TestUsersConfig_Validate(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		name   string
		modify func(*UsersConfig)
		exp    string
	}{
		{
			name:   "default config is valid",
			modify: nil,
			exp:    "",
		},
		{
			name: "missing min dynamic user",
			modify: func(u *UsersConfig) {
				u.MinDynamicUser = nil
			},
			exp: "dynamic_user_min must be set",
		},
		{
			name: "missing max dynamic user",
			modify: func(u *UsersConfig) {
				u.MaxDynamicUser = nil
			},
			exp: "dynamic_user_max must be set",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u := DefaultUsersConfig()
			if tc.modify != nil {
				tc.modify(u)
			}
			err := u.Validate()
			if tc.exp != "" {
				must.ErrorContains(t, err, tc.exp)
			} else {
				must.NoError(t, err)
			}
		})
	}
}
