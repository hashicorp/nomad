package structs

import "errors"

const (
	errPluginShutdown = "plugin is shut down"
)

var (
	// ErrPluginShutdown is returned when the plugin has shutdown.
	ErrPluginShutdown = errors.New(errPluginShutdown)
)
