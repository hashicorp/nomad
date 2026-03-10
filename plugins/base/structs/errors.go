// Copyright IBM Corp. 2015, 2025
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
