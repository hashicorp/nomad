// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package commonplugins

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
)

func TestExternalSecretsPlugin_Fingerprint(t *testing.T) {
	ci.Parallel(t)

	t.Run("runs successfully", func(t *testing.T) {
		pluginDir, pluginName := setupTestPlugin(t, fmt.Appendf([]byte{}, "#!/bin/sh\ncat <<EOF\n%s\nEOF\n", `{"type": "secrets", "version": "1.0.0"}`))

		plugin, err := NewExternalSecretsPlugin(pluginDir, pluginName)
		must.NoError(t, err)

		res, err := plugin.Fingerprint(context.Background())
		must.NoError(t, err)

		must.Eq(t, *res.Type, "secrets")
		must.Eq(t, res.Version.String(), "1.0.0")
	})

	t.Run("errors on non-zero exit code", func(t *testing.T) {
		pluginDir, pluginName := setupTestPlugin(t, fmt.Append([]byte{}, "#!/bin/sh\nexit 1\n"))

		plugin, err := NewExternalSecretsPlugin(pluginDir, pluginName)
		must.NoError(t, err)

		res, err := plugin.Fingerprint(context.Background())
		must.Error(t, err)
		must.Nil(t, res)
	})

	t.Run("errors on timeout", func(t *testing.T) {
		pluginDir, pluginName := setupTestPlugin(t, fmt.Appendf([]byte{}, "#!/bin/sh\nleep .5\n"))

		plugin, err := NewExternalSecretsPlugin(pluginDir, pluginName)
		must.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		_, err = plugin.Fingerprint(ctx)
		must.Error(t, err)
	})

	t.Run("errors on invalid json", func(t *testing.T) {
		pluginDir, pluginName := setupTestPlugin(t, fmt.Append([]byte{}, "#!/bin/sh\ncat <<EOF\ninvalid\nEOF\n"))

		plugin, err := NewExternalSecretsPlugin(pluginDir, pluginName)
		must.NoError(t, err)

		res, err := plugin.Fingerprint(context.Background())
		must.Error(t, err)
		must.Nil(t, res)
	})
}

func TestExternalSecretsPlugin_Fetch(t *testing.T) {
	ci.Parallel(t)

	t.Run("runs successfully", func(t *testing.T) {
		pluginDir, pluginName := setupTestPlugin(t, fmt.Appendf([]byte{}, "#!/bin/sh\ncat <<EOF\n%s\nEOF\n", `{"result": {"key": "value"}}`))

		plugin, err := NewExternalSecretsPlugin(pluginDir, pluginName)
		must.NoError(t, err)

		res, err := plugin.Fetch(context.Background(), "test-path", nil)
		must.NoError(t, err)

		exp := map[string]string{"key": "value"}
		must.Eq(t, res.Result, exp)
	})

	t.Run("errors on non-zero exit code", func(t *testing.T) {
		pluginDir, pluginName := setupTestPlugin(t, fmt.Append([]byte{}, "#!/bin/sh\nexit 1\n"))

		plugin, err := NewExternalSecretsPlugin(pluginDir, pluginName)
		must.NoError(t, err)

		_, err = plugin.Fetch(context.Background(), "test-path", nil)
		must.Error(t, err)
	})

	t.Run("errors on timeout", func(t *testing.T) {
		pluginDir, pluginName := setupTestPlugin(t, fmt.Append([]byte{}, "#!/bin/sh\nsleep .5\n"))

		plugin, err := NewExternalSecretsPlugin(pluginDir, pluginName)
		must.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		_, err = plugin.Fetch(ctx, "dummy-path", nil)
		must.Error(t, err)
	})

	t.Run("errors on timeout", func(t *testing.T) {
		pluginDir, pluginName := setupTestPlugin(t, fmt.Appendf([]byte{}, "#!/bin/sh\ncat <<EOF\n%s\nEOF\n", `invalid`))

		plugin, err := NewExternalSecretsPlugin(pluginDir, pluginName)
		must.NoError(t, err)

		_, err = plugin.Fetch(context.Background(), "dummy-path", nil)
		must.Error(t, err)
	})

	t.Run("can be passed environment variables via Fetch", func(t *testing.T) {
		// test the passed envVar is parsed and set correctly by printing it as part of the SecretResponse
		pluginDir, pluginName := setupTestPlugin(t, fmt.Appendf([]byte{}, "#!/bin/sh\ncat <<EOF\n%s\nEOF\n", `{"result": {"foo": "$TEST_KEY"}}`))

		plugin, err := NewExternalSecretsPlugin(pluginDir, pluginName)
		must.NoError(t, err)

		res, err := plugin.Fetch(context.Background(), "dummy-path", map[string]string{"TEST_KEY": "TEST_VALUE"})
		must.NoError(t, err)
		must.Eq(t, res.Result, map[string]string{"foo": "TEST_VALUE"})
	})
}

func TestExternalSecretsPlugin_CustomTimeout(t *testing.T) {
	ci.Parallel(t)

	t.Run("uses custom timeout when specified", func(t *testing.T) {
		// Plugin sleeps for 2 seconds
		pluginDir, pluginName := setupTestPlugin(t, fmt.Appendf([]byte{}, "#!/bin/sh\nsleep 2\ncat <<EOF\n%s\nEOF\n", `{"result": {"key": "value"}}`))

		// With 1 second timeout, should fail
		plugin, err := NewExternalSecretsPlugin(pluginDir, pluginName, WithTimeout(1*time.Second))
		must.NoError(t, err)

		_, err = plugin.Fetch(context.Background(), "test-path", nil)
		must.Error(t, err)
		must.ErrorContains(t, err, "signal: terminated")
	})

	t.Run("succeeds with sufficient timeout", func(t *testing.T) {
		// Plugin sleeps for 1 second
		pluginDir, pluginName := setupTestPlugin(t, fmt.Appendf([]byte{}, "#!/bin/sh\nsleep 1\ncat <<EOF\n%s\nEOF\n", `{"result": {"key": "value"}}`))

		// With 5 second timeout, should succeed
		plugin, err := NewExternalSecretsPlugin(pluginDir, pluginName, WithTimeout(5*time.Second))
		must.NoError(t, err)

		res, err := plugin.Fetch(context.Background(), "test-path", nil)
		must.NoError(t, err)

		exp := map[string]string{"key": "value"}
		must.Eq(t, res.Result, exp)
	})

	t.Run("defaults to 10s when no timeout specified", func(t *testing.T) {
		pluginDir, pluginName := setupTestPlugin(t, fmt.Appendf([]byte{}, "#!/bin/sh\ncat <<EOF\n%s\nEOF\n", `{"result": {"key": "value"}}`))

		// No timeout option, should use default 10s
		plugin, err := NewExternalSecretsPlugin(pluginDir, pluginName)
		must.NoError(t, err)

		res, err := plugin.Fetch(context.Background(), "test-path", nil)
		must.NoError(t, err)

		exp := map[string]string{"key": "value"}
		must.Eq(t, res.Result, exp)
	})
}

func setupTestPlugin(t *testing.T, b []byte) (string, string) {
	dir := t.TempDir()
	plugin := "test-plugin"

	// NewExternalSecretsPlugin expects the subdir "secrets" to be present
	secretDir := filepath.Join(dir, SecretsPluginDir)
	must.NoError(t, os.Mkdir(secretDir, 0755))

	path := filepath.Join(secretDir, plugin)
	must.NoError(t, os.WriteFile(path, b, 0755))

	return dir, plugin
}
