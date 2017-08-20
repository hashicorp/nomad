package integration

import (
	"log"
	"os"
	"testing"

	"github.com/hashicorp/nomad/client/drivernew/exec"
	"github.com/hashicorp/nomad/client/drivernew/plugin"
	"github.com/hashicorp/nomad/plugins/catalog"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/assert"
)

func testLogger() *log.Logger {
	return log.New(os.Stderr, "", log.LstdFlags)
}

func TestIntegration_Builtin_Plugin(t *testing.T) {
	if !testutil.IsNomadTest() {
		t.Skipf("Must be run with build flag %q", "nomad_test")
	}

	t.Parallel()
	assert := assert.New(t)
	c := &catalog.PluginIndex{}
	opts := &plugin.FactoryOpts{
		Name:    exec.Name,
		Catalog: c,
		Logger:  testLogger(),
	}
	d, reattach, err := plugin.PluginFactory(opts)
	assert.Nil(err)
	assert.NotNil(d)
	assert.NotNil(reattach)
	defer d.Exit()

	// Check that the driver is actually using go-plugin
	assert.IsType(&plugin.DriverPluginClient{}, d)

	// Get the name using an RPC
	act, err := d.Name()
	assert.Nil(err)
	assert.Equal(exec.Name, act)

	// Reattach to the plugin
	opts.ReattachConfig = reattach
	d2, r2, err := plugin.PluginFactory(opts)
	assert.Nil(err)
	assert.NotNil(r2)
	assert.NotNil(d2)
	assert.IsType(&plugin.DriverPluginClient{}, d)

	// Get the name using an RPC
	act2, err := d.Name()
	assert.Nil(err)
	assert.Equal(exec.Name, act2)
}

func TestIntegration_Builtin_Inprocess(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	opts := &plugin.FactoryOpts{
		Name:            exec.Name,
		Catalog:         &catalog.PluginIndex{},
		Logger:          testLogger(),
		PreferInprocess: true,
	}
	d, reattach, err := plugin.PluginFactory(opts)
	assert.Nil(err)
	assert.Nil(reattach)
	defer d.Exit()

	// Check that the driver is actually using go-plugin
	assert.IsType(&exec.Exec{}, d)

	// Get the name using an RPC
	act, err := d.Name()
	assert.Nil(err)
	assert.Equal(exec.Name, act)
}
