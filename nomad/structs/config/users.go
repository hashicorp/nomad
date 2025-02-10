// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package config

import (
	"errors"

	"github.com/hashicorp/nomad/helper/pointer"
)

// UsersConfig configures things related to operating system users.
type UsersConfig struct {
	// MinDynamicUser is the lowest uid/gid for use in the dynamic users pool.
	MinDynamicUser *int `hcl:"dynamic_user_min"`

	// MaxDynamicUser is the highest uid/gid for use in the dynamic users pool.
	MaxDynamicUser *int `hcl:"dynamic_user_max"`
}

// Copy returns a deep copy of the Users struct.
func (u *UsersConfig) Copy() *UsersConfig {
	if u == nil {
		return nil
	}
	return &UsersConfig{
		MinDynamicUser: pointer.Copy(u.MinDynamicUser),
		MaxDynamicUser: pointer.Copy(u.MaxDynamicUser),
	}
}

// Merge returns a new Users where non-empty/nil fields in the argument have
// higher precedence.
func (u *UsersConfig) Merge(o *UsersConfig) *UsersConfig {
	switch {
	case u == nil:
		return o.Copy()
	case o == nil:
		return u.Copy()
	default:
		return &UsersConfig{
			MinDynamicUser: pointer.Merge(u.MinDynamicUser, o.MinDynamicUser),
			MaxDynamicUser: pointer.Merge(u.MaxDynamicUser, o.MaxDynamicUser),
		}
	}
}

// Equal returns whether u and o are the same.
func (u *UsersConfig) Equal(o *UsersConfig) bool {
	if u == nil || o == nil {
		return u == o
	}
	switch {
	case !pointer.Eq(u.MinDynamicUser, o.MinDynamicUser):
		return false
	case !pointer.Eq(u.MaxDynamicUser, o.MaxDynamicUser):
		return false
	default:
		return true
	}
}

var (
	errUsersUnset            = errors.New("users must not be nil")
	errDynamicUserMinUnset   = errors.New("dynamic_user_min must be set")
	errDynamicUserMinInvalid = errors.New("dynamic_user_min must not be negative")
	errDynamicUserMaxUnset   = errors.New("dynamic_user_max must be set")
	errDynamicUserMaxInvalid = errors.New("dynamic_user_max must not be negative")
)

// Validate whether UsersConfig is valid.
//
// Note that -1 is a valid value for min/max dynamic users, as this is used
// to indicate the dynamic workload users feature should be disabled.
func (u *UsersConfig) Validate() error {
	if u == nil {
		return errUsersUnset
	}
	if u.MinDynamicUser == nil {
		return errDynamicUserMinUnset
	}
	if *u.MinDynamicUser < -1 {
		return errDynamicUserMinInvalid
	}
	if u.MaxDynamicUser == nil {
		return errDynamicUserMaxUnset
	}
	if *u.MaxDynamicUser < -1 {
		return errDynamicUserMaxInvalid
	}
	return nil
}

// DefaultUsersConfig returns the default users configuration.
func DefaultUsersConfig() *UsersConfig {
	return &UsersConfig{
		MinDynamicUser: pointer.Of(80_000),
		MaxDynamicUser: pointer.Of(89_999),
	}
}
