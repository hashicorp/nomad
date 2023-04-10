// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !release
// +build !release

package catalog

import "github.com/hashicorp/nomad/drivers/mock"

// Register the mock driver with the builtin driver plugin catalog. All builtin
// plugins that are intended for production use should be registered in
// register.go as this file is not built as part of a release.
func init() {
	Register(mock.PluginID, mock.PluginConfig)
}
