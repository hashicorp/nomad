// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package template

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"text/template"
	"time"

	ctconf "github.com/hashicorp/consul-template/config"
	ctmanager "github.com/hashicorp/consul-template/manager"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/commonplugins"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

// setupTestPlugin creates a test secrets plugin in a temporary directory.
// Returns the plugin directory and plugin name.
func setupTestPlugin(t *testing.T, pluginName string, script []byte) string {
	t.Helper()

	dir := t.TempDir()

	// NewExternalSecretsPlugin expects the subdir "secrets" to be present
	secretDir := filepath.Join(dir, commonplugins.SecretsPluginDir)
	must.NoError(t, os.Mkdir(secretDir, 0755))

	path := filepath.Join(secretDir, pluginName)
	must.NoError(t, os.WriteFile(path, script, 0755))

	return dir
}

func TestSecretFuncs_Registration(t *testing.T) {
	ci.Parallel(t)

	cfg := &nomadSecretConfig{
		CommonPluginDir: "/nonexistent/plugins",
		Namespace:       "default",
		JobID:           "test-job",
		Secrets:         []*structs.Secret{},
	}

	funcs := nomadSecretFuncs(cfg)

	// Verify the nomadSecret function is registered
	_, ok := funcs["nomadSecret"]
	must.True(t, ok, must.Sprint("nomadSecret function should be registered"))
}

func TestSecretFunc_EmptySecretName(t *testing.T) {
	ci.Parallel(t)

	cfg := &nomadSecretConfig{
		CommonPluginDir: "/nonexistent/plugins",
		Namespace:       "default",
		JobID:           "test-job",
		Secrets:         []*structs.Secret{},
	}

	fn := nomadSecretFunc(cfg)

	_, err := fn("")
	must.Error(t, err)
	must.StrContains(t, err.Error(), "secret name is required")
}

func TestSecretFunc_SecretNotFound(t *testing.T) {
	ci.Parallel(t)

	cfg := &nomadSecretConfig{
		CommonPluginDir: "/nonexistent/plugins",
		Namespace:       "default",
		JobID:           "test-job",
		Secrets: []*structs.Secret{
			{Name: "existing_secret", Provider: "test", Path: "/path"},
		},
	}

	fn := nomadSecretFunc(cfg)

	_, err := fn("nonexistent_secret")
	must.Error(t, err)
	must.StrContains(t, err.Error(), "secret \"nonexistent_secret\" not found in task configuration")
}

func TestSecretFunc_PluginNotFound(t *testing.T) {
	ci.Parallel(t)

	cfg := &nomadSecretConfig{
		CommonPluginDir: "/nonexistent/plugins",
		Namespace:       "default",
		JobID:           "test-job",
		Secrets: []*structs.Secret{
			{Name: "my_secret", Provider: "nonexistent-plugin", Path: "/path"},
		},
	}

	fn := nomadSecretFunc(cfg)

	_, err := fn("my_secret")
	must.Error(t, err)
	must.StrContains(t, err.Error(), "failed to create secrets plugin")
}

func TestSecretFunc_SuccessfulFetch(t *testing.T) {
	ci.Parallel(t)

	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}

	// Create a mock plugin that returns secrets
	script := []byte(`#!/bin/sh
cat <<EOF
{"result": {"username": "AzureDiamond", "password": "hunter2"}}
EOF
`)
	pluginDir := setupTestPlugin(t, "test-plugin", script)

	cfg := &nomadSecretConfig{
		CommonPluginDir: pluginDir,
		Namespace:       "default",
		JobID:           "test-job",
		Secrets: []*structs.Secret{
			{Name: "db_creds", Provider: "test-plugin", Path: "/db/creds"},
		},
	}

	fn := nomadSecretFunc(cfg)

	result, err := fn("db_creds")
	must.NoError(t, err)
	must.NotNil(t, result)

	// Verify the returned map contains expected values
	must.Eq(t, "AzureDiamond", result["username"])
	must.Eq(t, "hunter2", result["password"])
	must.Eq(t, 2, len(result))
}

func TestSecretFunc_PluginReturnsError(t *testing.T) {
	ci.Parallel(t)

	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}

	// Create a mock plugin that returns an error in the response
	script := []byte(`#!/bin/sh
cat <<EOF
{"result": null, "error": "secret not found"}
EOF
`)
	pluginDir := setupTestPlugin(t, "test-plugin", script)

	cfg := &nomadSecretConfig{
		CommonPluginDir: pluginDir,
		Namespace:       "default",
		JobID:           "test-job",
		Secrets: []*structs.Secret{
			{Name: "missing_secret", Provider: "test-plugin", Path: "/nonexistent/path"},
		},
	}

	fn := nomadSecretFunc(cfg)

	_, err := fn("missing_secret")
	must.Error(t, err)
	must.StrContains(t, err.Error(), "secret not found")
}

func TestSecretFunc_PluginExitsNonZero(t *testing.T) {
	ci.Parallel(t)

	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}

	// Create a mock plugin that exits with error
	script := []byte(`#!/bin/sh
exit 1
`)
	pluginDir := setupTestPlugin(t, "test-plugin", script)

	cfg := &nomadSecretConfig{
		CommonPluginDir: pluginDir,
		Namespace:       "default",
		JobID:           "test-job",
		Secrets: []*structs.Secret{
			{Name: "my_secret", Provider: "test-plugin", Path: "/path"},
		},
	}

	fn := nomadSecretFunc(cfg)

	_, err := fn("my_secret")
	must.Error(t, err)
	must.StrContains(t, err.Error(), "failed to fetch secret")
}

func TestSecretFunc_PluginReturnsInvalidJSON(t *testing.T) {
	ci.Parallel(t)

	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}

	// Create a mock plugin that returns invalid JSON
	script := []byte(`#!/bin/sh
echo "not valid json"
`)
	pluginDir := setupTestPlugin(t, "test-plugin", script)

	cfg := &nomadSecretConfig{
		CommonPluginDir: pluginDir,
		Namespace:       "default",
		JobID:           "test-job",
		Secrets: []*structs.Secret{
			{Name: "my_secret", Provider: "test-plugin", Path: "/path"},
		},
	}

	fn := nomadSecretFunc(cfg)

	_, err := fn("my_secret")
	must.Error(t, err)
	must.StrContains(t, err.Error(), "failed to fetch secret")
}

func TestSecretFunc_EmptyResult(t *testing.T) {
	ci.Parallel(t)

	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}

	// Create a mock plugin that returns empty result
	script := []byte(`#!/bin/sh
cat <<EOF
{"result": {}}
EOF
`)
	pluginDir := setupTestPlugin(t, "test-plugin", script)

	cfg := &nomadSecretConfig{
		CommonPluginDir: pluginDir,
		Namespace:       "default",
		JobID:           "test-job",
		Secrets: []*structs.Secret{
			{Name: "empty_secret", Provider: "test-plugin", Path: "/empty/path"},
		},
	}

	fn := nomadSecretFunc(cfg)

	result, err := fn("empty_secret")
	must.NoError(t, err)
	must.NotNil(t, result)
	must.Eq(t, 0, len(result))
}

func TestSecretFunc_EnvironmentVariablesFromSecretBlock(t *testing.T) {
	ci.Parallel(t)

	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}

	// Create a mock plugin that echoes the environment variables
	script := []byte(`#!/bin/sh
cat <<EOF
{"result": {"aws_region": "$AWS_REGION", "custom_var": "$CUSTOM_VAR", "namespace": "$NOMAD_NAMESPACE", "job_id": "$NOMAD_JOB_ID"}}
EOF
`)
	pluginDir := setupTestPlugin(t, "aws-ssm", script)

	cfg := &nomadSecretConfig{
		CommonPluginDir: pluginDir,
		Namespace:       "my-namespace",
		JobID:           "my-job-id",
		Secrets: []*structs.Secret{
			{
				Name:     "app_secrets",
				Provider: "aws-ssm",
				Path:     "/test/nomad/app",
				Env: map[string]string{
					"AWS_REGION": "us-east-1",
					"CUSTOM_VAR": "custom-value",
				},
			},
		},
	}

	fn := nomadSecretFunc(cfg)

	result, err := fn("app_secrets")
	must.NoError(t, err)
	must.Eq(t, "us-east-1", result["aws_region"])
	must.Eq(t, "custom-value", result["custom_var"])
	must.Eq(t, "my-namespace", result["namespace"])
	must.Eq(t, "my-job-id", result["job_id"])
}

func TestSecretFunc_MultipleSecrets(t *testing.T) {
	ci.Parallel(t)

	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}

	// Create mock plugins
	dbScript := []byte(`#!/bin/sh
cat <<EOF
{"result": {"host": "db.example.com", "port": "5432"}}
EOF
`)
	apiScript := []byte(`#!/bin/sh
cat <<EOF
{"result": {"api_key": "sk-12345", "api_secret": "secret-abc"}}
EOF
`)
	pluginDir := setupTestPlugin(t, "db-plugin", dbScript)
	// Add second plugin
	secretDir := filepath.Join(pluginDir, commonplugins.SecretsPluginDir)
	must.NoError(t, os.WriteFile(filepath.Join(secretDir, "api-plugin"), apiScript, 0755))

	cfg := &nomadSecretConfig{
		CommonPluginDir: pluginDir,
		Namespace:       "default",
		JobID:           "test-job",
		Secrets: []*structs.Secret{
			{Name: "database", Provider: "db-plugin", Path: "/db/config"},
			{Name: "api_creds", Provider: "api-plugin", Path: "/api/keys"},
		},
	}

	fn := nomadSecretFunc(cfg)

	// Fetch first secret
	dbResult, err := fn("database")
	must.NoError(t, err)
	must.Eq(t, "db.example.com", dbResult["host"])
	must.Eq(t, "5432", dbResult["port"])

	// Fetch second secret
	apiResult, err := fn("api_creds")
	must.NoError(t, err)
	must.Eq(t, "sk-12345", apiResult["api_key"])
	must.Eq(t, "secret-abc", apiResult["api_secret"])
}

func TestSecretFunc_SpecialCharactersInValues(t *testing.T) {
	ci.Parallel(t)

	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}

	// Create a mock plugin that returns values with special characters
	script := []byte(`#!/bin/sh
cat <<'EOF'
{"result": {"password": "p@ss=word\"with'special", "url": "https://example.com?foo=bar&baz=qux"}}
EOF
`)
	pluginDir := setupTestPlugin(t, "test-plugin", script)

	cfg := &nomadSecretConfig{
		CommonPluginDir: pluginDir,
		Namespace:       "default",
		JobID:           "test-job",
		Secrets: []*structs.Secret{
			{Name: "special_chars", Provider: "test-plugin", Path: "/special/chars"},
		},
	}

	fn := nomadSecretFunc(cfg)

	result, err := fn("special_chars")
	must.NoError(t, err)
	must.Eq(t, `p@ss=word"with'special`, result["password"])
	must.Eq(t, "https://example.com?foo=bar&baz=qux", result["url"])
}

func TestNomadSecretItems_Iteration(t *testing.T) {
	ci.Parallel(t)

	// Test that NomadSecretItems can be iterated over
	items := NomadSecretItems{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}

	// Count iterations
	count := 0
	for k, v := range items {
		count++
		must.NotEq(t, "", k)
		must.NotEq(t, "", v)
	}
	must.Eq(t, 3, count)
}

func TestSecretFunc_TemplateIntegration(t *testing.T) {
	ci.Parallel(t)

	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}

	// Create a mock plugin
	script := []byte(`#!/bin/sh
cat <<EOF
{"result": {"DB_HOST": "localhost", "DB_PORT": "5432", "DB_USER": "admin"}}
EOF
`)
	pluginDir := setupTestPlugin(t, "test-plugin", script)

	cfg := &nomadSecretConfig{
		CommonPluginDir: pluginDir,
		Namespace:       "default",
		JobID:           "test-job",
		Secrets: []*structs.Secret{
			{Name: "db_config", Provider: "test-plugin", Path: "/db/config"},
		},
	}

	// Create a template that uses secret
	tmplText := `{{ range $k, $v := nomadSecret "db_config" }}
{{ $k }}={{ $v }}
{{ end }}`

	tmpl, err := template.New("test").Funcs(nomadSecretFuncs(cfg)).Parse(tmplText)
	must.NoError(t, err)

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, nil)
	must.NoError(t, err)

	output := buf.String()
	must.StrContains(t, output, "DB_HOST=localhost")
	must.StrContains(t, output, "DB_PORT=5432")
	must.StrContains(t, output, "DB_USER=admin")
}

func TestSecretFunc_TemplateWithClause(t *testing.T) {
	ci.Parallel(t)

	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}

	// Create a mock plugin
	script := []byte(`#!/bin/sh
cat <<EOF
{"result": {"host": "db.example.com", "port": "3306"}}
EOF
`)
	pluginDir := setupTestPlugin(t, "test-plugin", script)

	cfg := &nomadSecretConfig{
		CommonPluginDir: pluginDir,
		Namespace:       "default",
		JobID:           "test-job",
		Secrets: []*structs.Secret{
			{Name: "mysql_creds", Provider: "test-plugin", Path: "/mysql/creds"},
		},
	}

	// Create a template that uses with clause and index
	tmplText := `{{ with nomadSecret "mysql_creds" }}host={{ index . "host" }}:{{ index . "port" }}{{ end }}`

	tmpl, err := template.New("test").Funcs(nomadSecretFuncs(cfg)).Parse(tmplText)
	must.NoError(t, err)

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, nil)
	must.NoError(t, err)

	must.Eq(t, "host=db.example.com:3306", buf.String())
}

func TestSecretFunc_TemplateErrorHandling(t *testing.T) {
	ci.Parallel(t)

	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}

	// Create a mock plugin that returns an error
	script := []byte(`#!/bin/sh
cat <<EOF
{"result": null, "error": "access denied"}
EOF
`)
	pluginDir := setupTestPlugin(t, "test-plugin", script)

	cfg := &nomadSecretConfig{
		CommonPluginDir: pluginDir,
		Namespace:       "default",
		JobID:           "test-job",
		Secrets: []*structs.Secret{
			{Name: "denied_secret", Provider: "test-plugin", Path: "/denied/path"},
		},
	}

	// Create a template that uses secret
	tmplText := `{{ nomadSecret "denied_secret" }}`

	tmpl, err := template.New("test").Funcs(nomadSecretFuncs(cfg)).Parse(tmplText)
	must.NoError(t, err)

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, nil)
	must.Error(t, err)
	must.StrContains(t, err.Error(), "access denied")
}

func TestSecretFunc_NilSecrets(t *testing.T) {
	ci.Parallel(t)

	// Test with nil secrets slice
	cfg := &nomadSecretConfig{
		CommonPluginDir: "/nonexistent",
		Secrets:         nil,
	}

	fn := nomadSecretFunc(cfg)

	_, err := fn("any_secret")
	must.Error(t, err)
	must.StrContains(t, err.Error(), "not found in task configuration")
}

func TestSecretFunc_PathPassedToPlugin(t *testing.T) {
	ci.Parallel(t)

	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}

	// Create a mock plugin that echoes the path argument
	// The plugin receives path as $2 (first arg is "fetch", second is path)
	script := []byte(`#!/bin/sh
# $1 is "fetch", $2 is the path
cat <<EOF
{"result": {"path_received": "$2"}}
EOF
`)
	pluginDir := setupTestPlugin(t, "test-plugin", script)

	cfg := &nomadSecretConfig{
		CommonPluginDir: pluginDir,
		Namespace:       "default",
		JobID:           "test-job",
		Secrets: []*structs.Secret{
			{Name: "my_secret", Provider: "test-plugin", Path: "/my/custom/secret/path"},
		},
	}

	fn := nomadSecretFunc(cfg)

	result, err := fn("my_secret")
	must.NoError(t, err)
	must.Eq(t, "/my/custom/secret/path", result["path_received"])
}

func TestSecretFunc_RealWorldAWSSSMExample(t *testing.T) {
	ci.Parallel(t)

	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}

	// Simulate an AWS SSM plugin response
	script := []byte(`#!/bin/sh
cat <<EOF
{"result": {"DB_HOST": "prod-db.internal", "DB_PORT": "5432", "DB_USER": "app_user", "DB_PASS": "hunter2"}}
EOF
`)
	pluginDir := setupTestPlugin(t, "aws-ssm", script)

	cfg := &nomadSecretConfig{
		CommonPluginDir: pluginDir,
		Namespace:       "default",
		JobID:           "test-job",
		Secrets: []*structs.Secret{
			{
				Name:     "app_secrets",
				Provider: "aws-ssm",
				Path:     "/prod/web-app/db",
				Env: map[string]string{
					"AWS_REGION":            "us-east-1",
					"AWS_ACCESS_KEY_ID":     "test",
					"AWS_SECRET_ACCESS_KEY": "test",
				},
			},
		},
	}

	// Test template similar to what a user would write
	tmplText := `{{ with nomadSecret "app_secrets" }}
DATABASE_URL=postgres://{{ index . "DB_USER" }}:{{ index . "DB_PASS" }}@{{ index . "DB_HOST" }}:{{ index . "DB_PORT" }}/mydb
{{ end }}`

	tmpl, err := template.New("test").Funcs(nomadSecretFuncs(cfg)).Parse(tmplText)
	must.NoError(t, err)

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, nil)
	must.NoError(t, err)

	must.StrContains(t, buf.String(), "DATABASE_URL=postgres://app_user:hunter2@prod-db.internal:5432/mydb")
}

func TestSecretFunc_ConsulTemplateRunner(t *testing.T) {
	ci.Parallel(t)

	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}

	// Create a mock plugin
	script := []byte(`#!/bin/sh
cat <<EOF
{"result": {"username": "testuser", "password": "testpass"}}
EOF
`)
	pluginDir := setupTestPlugin(t, "test-plugin", script)

	secrets := []*structs.Secret{
		{Name: "my_secret", Provider: "test-plugin", Path: "/test/path"},
	}

	// Create the ExtFuncMap
	extFuncMap := nomadSecretFuncs(&nomadSecretConfig{
		CommonPluginDir: pluginDir,
		Secrets:         secrets,
	})

	// Template content using our function
	tmplContent := `{{ with nomadSecret "my_secret" }}user={{ index . "username" }}{{ end }}`

	// Create destination file
	destPath := filepath.Join(t.TempDir(), "output.txt")

	// Create consul-template TemplateConfig with ExtFuncMap
	tc := ctconf.DefaultTemplateConfig()
	tc.Contents = &tmplContent
	tc.Destination = &destPath
	tc.ExtFuncMap = extFuncMap
	tc.Finalize()

	// Create consul-template Config
	cfg := ctconf.DefaultConfig()
	cfg.Once = true
	cfg.Templates = &ctconf.TemplateConfigs{tc}
	cfg.Finalize()

	// Create and run the runner
	runner, err := ctmanager.NewRunner(cfg, false)
	must.NoError(t, err)

	go runner.Start()
	defer runner.Stop()

	select {
	case <-runner.DoneCh:
		// Success
	case err := <-runner.ErrCh:
		t.Fatalf("runner error: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for runner")
	}

	// Read the output
	content, err := os.ReadFile(destPath)
	must.NoError(t, err)
	must.Eq(t, "user=testuser", string(content))
}
