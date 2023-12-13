// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package pluginmanager

import "context"

// PluginManager orchestrates the lifecycle of a set of plugins
type PluginManager interface {
	// Run starts a plugin manager and should return early
	Run()

	// Shutdown should gracefully shutdown all plugins managed by the manager.
	// It must block until shutdown is complete
	Shutdown()

	// PluginType is the type of plugin which the manager manages
	PluginType() string
}

// FingerprintingPluginManager is an interface that exposes fingerprinting
// coordination for plugin managers
type FingerprintingPluginManager interface {
	PluginManager

	// WaitForFirstFingerprint returns a channel that is closed once all plugin
	// instances managed by the plugin manager have fingerprinted once. A
	// context can be passed which when done will also close the channel
	WaitForFirstFingerprint(context.Context) <-chan struct{}
}
