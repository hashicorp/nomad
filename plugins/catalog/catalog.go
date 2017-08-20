package catalog

import (
	"fmt"
	"sync"

	"github.com/hashicorp/nomad/helper/discover"
	"github.com/hashicorp/nomad/plugins/builtin"
	"github.com/hashicorp/nomad/plugins/runner"
	"github.com/hashicorp/nomad/plugins/types"
)

type PluginCatalog interface {
	Get(t types.PluginType, name string) (*runner.PluginRunner, error)
	Add(t types.PluginType, name, command string) error // TODO figure out configs
	Delete(t types.PluginType, name string) error
	List() (map[types.PluginType][]string, error)
}

type PluginIndex struct {
	plugins       map[types.PluginType]map[string]*runner.PluginRunner
	nomadExe      string
	nomadExeError error
	l             sync.RWMutex
}

func (p *PluginIndex) Get(t types.PluginType, name string) (*runner.PluginRunner, error) {
	p.l.RLock()
	defer p.l.RUnlock()

	// First check if we have a plugin by type/name and then check the built in
	// map
	if plugin, ok := p.plugins[t][name]; ok {
		return plugin, nil
	}

	if runner := builtin.GetRunner(t, name); runner != nil {
		// Get the Nomad executable
		if runner.Command == "" && p.nomadExe == "" && p.nomadExeError == nil {
			p.nomadExe, p.nomadExeError = discover.NomadExecutable()
		}

		if p.nomadExeError != nil {
			return nil, p.nomadExeError
		} else if runner.Command == "" && p.nomadExe == "" {
			return nil, fmt.Errorf("failed to discover Nomad executable")
		} else if runner.Command == "" {
			runner.Command = p.nomadExe
		}

		return runner, nil
	}

	return nil, fmt.Errorf("no plugin of type %q with name %q", t, name)
}

func (p *PluginIndex) Add(t types.PluginType, name, command string) error {
	p.l.Lock()
	defer p.l.Unlock()

	// Create the runner
	r := &runner.PluginRunner{
		Name:    name,
		Command: command,
		Type:    t,
	}

	if p.plugins == nil {
		p.plugins = make(map[types.PluginType]map[string]*runner.PluginRunner)
	}

	typedPlugins, ok := p.plugins[t]
	if !ok {
		typedPlugins = make(map[string]*runner.PluginRunner)
		p.plugins[t] = typedPlugins
	}
	typedPlugins[name] = r
	return nil
}

func (p *PluginIndex) Delete(t types.PluginType, name string) error {
	p.l.Lock()
	defer p.l.Unlock()

	// First try to delete from added plugins and then check to make sure the
	// delete request is not for a builtin plugin.
	if _, ok := p.plugins[t][name]; ok {
		delete(p.plugins[t], name)
		return nil
	}

	if _, ok := builtin.Plugins[t][name]; ok {
		return fmt.Errorf("can not delete builtin plugin")
	}

	return fmt.Errorf("unknown plugin %q of type %s", name, t)
}

func (p *PluginIndex) List() (map[types.PluginType][]string, error) {
	p.l.RLock()
	defer p.l.RUnlock()

	out := make(map[types.PluginType][]string, len(p.plugins))
	set := make(map[types.PluginType]map[string]struct{}, len(p.plugins))

	all := []map[types.PluginType]map[string]*runner.PluginRunner{builtin.Plugins, p.plugins}
	for _, plugins := range all {
		for ptype, plugs := range plugins {
			names, ok := set[ptype]
			if !ok {
				names = make(map[string]struct{})
				set[ptype] = names
			}

			for name := range plugs {
				names[name] = struct{}{}
			}
		}
	}

	for ptype, names := range set {
		list := make([]string, 0, len(names))
		for name := range names {
			list = append(list, name)
		}
		out[ptype] = list
	}

	return out, nil
}
