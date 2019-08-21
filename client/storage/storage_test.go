package storage

import (
	"testing"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/stretchr/testify/require"
)

func TestStorageCatalog_External(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	// Create a plugin
	plugins := map[string]*PluginConfig{
		"mock-csi": &PluginConfig{
			Address: "/tmp/non-existent.sock",
		},
	}

	logger := testlog.HCLogger(t)
	logger.SetLevel(hclog.Trace)

	l := NewPluginLoader(logger, plugins)

	// Get the catalog and assert we have the a plugin
	c := l.Configs()
	require.Len(c, 1)
	require.Contains(c, "mock-csi")

	// Vendor a plugin client for mock-csi
	p, err := l.Dispense("mock-csi")
	require.NoError(err)
	require.NotNil(p)
	p.Close()

	// Try vendoring a plugin client for a plugin that doesn't exist
	p, err = l.Dispense("non-existent")
	require.Error(PluginNotFoundErr, err)
	require.Nil(p)
}
