// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package config

import "github.com/mitchellh/copystructure"

// PluginConfig is used to configure a plugin explicitly
type PluginConfig struct {
	Name   string                 `hcl:",key"`
	Args   []string               `hcl:"args"`
	Config map[string]interface{} `hcl:"config"`
	// ExtraKeysHCL is used by hcl to surface unexpected keys
	ExtraKeysHCL []string `hcl:",unusedKeys" json:"-"`
}

func (p *PluginConfig) Merge(o *PluginConfig) *PluginConfig {
	m := *p

	if len(o.Name) != 0 {
		m.Name = o.Name
	}
	if len(o.Args) != 0 {
		m.Args = o.Args
	}
	if len(o.Config) != 0 {
		m.Config = o.Config
	}

	return m.Copy()
}

func (p *PluginConfig) Copy() *PluginConfig {
	c := *p
	if i, err := copystructure.Copy(p.Config); err != nil {
		panic(err.Error())
	} else {
		c.Config = i.(map[string]interface{})
	}
	return &c
}

// PluginConfigSetMerge merges to sets of plugin configs. For plugins with the
// same name, the configs are merged.
func PluginConfigSetMerge(first, second []*PluginConfig) []*PluginConfig {
	findex := make(map[string]*PluginConfig, len(first))
	for _, p := range first {
		findex[p.Name] = p
	}

	sindex := make(map[string]*PluginConfig, len(second))
	for _, p := range second {
		sindex[p.Name] = p
	}

	var out []*PluginConfig

	// Go through the first set and merge any value that exist in both
	for pluginName, original := range findex {
		second, ok := sindex[pluginName]
		if !ok {
			out = append(out, original.Copy())
			continue
		}

		out = append(out, original.Merge(second))
	}

	// Go through the second set and add any value that didn't exist in both
	for pluginName, plugin := range sindex {
		_, ok := findex[pluginName]
		if ok {
			continue
		}

		out = append(out, plugin)
	}

	return out
}
