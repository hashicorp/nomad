// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package config

import sconfig "github.com/hashicorp/nomad/nomad/structs/config"

// UsersConfig configures things related to operating system users.
type UsersConfig struct {
	// MinDynamicUser is the lowest uid/gid for use in the dynamic users pool.
	MinDynamicUser int

	// MaxDynamicUser is the highest uid/gid for use in the dynamic users pool.
	MaxDynamicUser int
}

func UsersConfigFromAgent(c *sconfig.UsersConfig) *UsersConfig {
	return &UsersConfig{
		MinDynamicUser: *c.MinDynamicUser,
		MaxDynamicUser: *c.MaxDynamicUser,
	}
}

func (u *UsersConfig) Copy() *UsersConfig {
	if u == nil {
		return nil
	}
	return &UsersConfig{
		MinDynamicUser: u.MinDynamicUser,
		MaxDynamicUser: u.MaxDynamicUser,
	}
}
