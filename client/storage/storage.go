package storage

import (
	hclog "github.com/hashicorp/go-hclog"
)

type PluginConfig struct {
	Address string
}

type PluginCatalog interface {
	Configs() map[string]*PluginConfig
}

type StoragePlugin interface {
}

func NewPluginLoader(logger hclog.Logger, configurations map[string]*PluginConfig) PluginCatalog {
	return &catalog{logger: logger.Named("storage_plugins"), configs: configurations}
}

type catalog struct {
	logger  hclog.Logger
	configs map[string]*PluginConfig
}

func (c *catalog) Configs() map[string]*PluginConfig {
	return c.configs
}
