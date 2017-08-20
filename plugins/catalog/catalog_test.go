package catalog

import (
	"testing"

	"github.com/hashicorp/nomad/client/drivernew/exec"
	"github.com/hashicorp/nomad/plugins/types"
	"github.com/stretchr/testify/assert"
)

func TestCatalog_IsPluginCatalog(t *testing.T) {
	var _ PluginCatalog = &PluginIndex{}
}

func TestCatalog_Builtin(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	p := &PluginIndex{}

	// Get a builtin plugin
	runner, err := p.Get(types.Driver, exec.Name)
	assert.Nil(err)
	assert.NotNil(runner)
	assert.True(runner.Builtin)
	assert.NotNil(runner.BuiltinFactory)

	// Get an unknown plugin
	_, err = p.Get(types.Driver, "___123_fooo")
	assert.NotNil(err)

	// List builtin plugins
	l, err := p.List()
	assert.Nil(err)
	assert.NotNil(l)

	drivers := l[types.Driver]
	assert.NotEmpty(drivers)
	found := false
	for _, d := range drivers {
		if d == exec.Name {
			found = true
			break
		}
	}
	assert.True(found)

	// Delete a builtin plugin
	assert.NotNil(p.Delete(types.Driver, exec.Name))
}

func TestCatalog_Plugins(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	p := &PluginIndex{}

	// Add a new plugin
	name := "foo"
	c := "bar"
	assert.Nil(p.Add(types.Driver, name, c))

	// Get the plugin
	runner, err := p.Get(types.Driver, name)
	assert.Nil(err)
	assert.NotNil(runner)
	assert.False(runner.Builtin)
	assert.Nil(runner.BuiltinFactory)
	assert.Equal(name, runner.Name)
	assert.Equal(c, runner.Command)
	assert.Equal(types.Driver, runner.Type)

	// Get an unknown plugin
	_, err = p.Get(types.Driver, "___123_fooo")
	assert.NotNil(err)

	// List builtin plugins
	l, err := p.List()
	assert.Nil(err)
	assert.NotNil(l)

	drivers := l[types.Driver]
	assert.NotEmpty(drivers)
	found := false
	for _, d := range drivers {
		if d == name {
			found = true
			break
		}
	}
	assert.True(found)

	// Delete the plugin
	assert.Nil(p.Delete(types.Driver, name))
}

func TestCatalog_OverrideBuiltin(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	p := &PluginIndex{}

	// Add a new plugin
	c := "bar"
	assert.Nil(p.Add(types.Driver, exec.Name, c))

	// Get the plugin
	runner, err := p.Get(types.Driver, exec.Name)
	assert.Nil(err)
	assert.NotNil(runner)
	assert.False(runner.Builtin)
	assert.Nil(runner.BuiltinFactory)
	assert.Equal(exec.Name, runner.Name)
	assert.Equal(c, runner.Command)
	assert.Equal(types.Driver, runner.Type)

	// List builtin plugins
	l, err := p.List()
	assert.Nil(err)
	assert.NotNil(l)

	drivers := l[types.Driver]
	assert.NotEmpty(drivers)
	found := false
	for _, d := range drivers {
		if d == exec.Name {
			found = true
			break
		}
	}
	assert.True(found)

	// Delete the plugin
	assert.Nil(p.Delete(types.Driver, exec.Name))
}
