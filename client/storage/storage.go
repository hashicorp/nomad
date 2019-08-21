package storage

import (
	"fmt"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/storage/csi"
)

type PluginConfig struct {
	Address string
}

type PluginCatalog interface {
	Configs() map[string]*PluginConfig
	Dispense(name string) (csi.Client, error)
}

func NewPluginLoader(logger hclog.Logger, configurations map[string]*PluginConfig) PluginCatalog {
	return &catalog{logger: logger.Named("storage_plugins"), configs: configurations}
}

var PluginNotFoundErr = fmt.Errorf("plugin not found")

type catalog struct {
	logger  hclog.Logger
	configs map[string]*PluginConfig
}

func (c *catalog) Configs() map[string]*PluginConfig {
	return c.configs
}

func (c *catalog) Dispense(name string) (csi.Client, error) {
	cfg, ok := c.configs[name]
	if !ok {
		return nil, PluginNotFoundErr
	}

	return csi.NewClient(cfg.Address)
}
