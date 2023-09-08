// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package utils

// IsUnixRoot returns true if system is a unix system and the effective uid of user is root
func IsUnixRoot() bool {
	return false
}
