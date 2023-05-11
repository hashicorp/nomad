// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package structs

import "errors"

const (
	errPluginShutdown = "plugin is shut down"
)

var (
	// ErrPluginShutdown is returned when the plugin has shutdown.
	ErrPluginShutdown = errors.New(errPluginShutdown)
)
