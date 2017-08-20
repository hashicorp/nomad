package plugin

import (
	"fmt"
	"log"

	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/plugins/catalog"
	"github.com/hashicorp/nomad/plugins/types"
)

// FactoryOpts is used to pass options into the plugin factory.
type FactoryOpts struct {
	// Name is the name of the plugin to create
	Name string

	// Catalog is the mechanism to lookup the plugin.
	Catalog catalog.PluginCatalog

	// Logger is the logger to use for the plugin client.
	Logger *log.Logger

	// PreferInprocess instructs the factory to prefer creating an in-proces
	// version rather than using go-plugin.
	PreferInprocess bool

	// ReattachConfig is used to reattach to a plugin rather than create a new
	// one.
	ReattachConfig *plugin.ReattachConfig
}

// PluginFactory is used to build a driver type plugin. If preferInprocess is
// set and the driver is builtin, an inprocess version will be returned.
func PluginFactory(opts *FactoryOpts) (Driver, *plugin.ReattachConfig, error) {
	runner, err := opts.Catalog.Get(types.Driver, opts.Name)
	if err != nil {
		return nil, nil, err
	}

	// The driver is built-in and we were asked to run in process, so just
	// instantiate it.
	if runner.Builtin && opts.PreferInprocess {
		// Plugin is builtin so we can retrieve an instance of the interface
		// from the pluginRunner. Then cast it to a Driver.
		driverRaw, err := runner.BuiltinFactory()
		if err != nil {
			return nil, nil, fmt.Errorf("error getting plugin type: %s", err)
		}

		d, ok := driverRaw.(Driver)
		if !ok {
			return nil, nil, fmt.Errorf("unsuported driver type: %s", opts.Name)
		}

		return d, nil, nil
	}

	// Run the driver using go-plugin
	d, reattach, err := newPluginClient(runner, opts.ReattachConfig, opts.Logger)
	if err != nil {
		return nil, nil, err
	}

	return d, reattach, nil
}
