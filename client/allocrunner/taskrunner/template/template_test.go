// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package template

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	templateconfig "github.com/hashicorp/consul-template/config"
	ctestutil "github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/allocdir"
	trtesting "github.com/hashicorp/nomad/client/allocrunner/taskrunner/testing"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/taskenv"
	clienttestutil "github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/users"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	sconfig "github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/hashicorp/nomad/testutil"
	"github.com/kr/pretty"
	"github.com/shoenig/test"
	"github.com/shoenig/test/must"
)

// TestMain overrides the normal top-level test runner for this package. When
// template-render subprocesses are run, they use os.Executable to find their
// own binary, which is the template.test binary when these tests are
// running. That causes the template-render subprocess to run all these tests!
// Bail out early if we know we're in the subprocess.
func TestMain(m *testing.M) {
	flag.Parse()
	if slices.Contains(flag.Args(), "template-render") {
		return
	}
	os.Exit(m.Run())
}

const (
	// TestTaskName is the name of the injected task. It should appear in the
	// environment variable $NOMAD_TASK_NAME
	TestTaskName = "test-task"
)

// testHarness is used to test the TaskTemplateManager by spinning up
// Consul/Vault as needed
type testHarness struct {
	manager        *TaskTemplateManager
	mockHooks      *trtesting.MockTaskHooks
	templates      []*structs.Template
	envBuilder     *taskenv.Builder
	node           *structs.Node
	config         *config.Config
	vaultToken     string
	taskDir        string
	vault          *testutil.TestVault
	consul         *ctestutil.TestServer
	emitRate       time.Duration
	nomadNamespace string
}

// newTestHarness returns a harness starting a dev consul and vault server,
// building the appropriate config and creating a TaskTemplateManager
func newTestHarness(t *testing.T, templates []*structs.Template, consul, vault bool) *testHarness {
	t.Helper()
	region := "global"
	mockNode := mock.Node()

	harness := &testHarness{
		mockHooks: trtesting.NewMockTaskHooks(),
		templates: templates,
		node:      mockNode,
		config: &config.Config{
			Node:   mockNode,
			Region: region,
			TemplateConfig: &config.ClientTemplateConfig{
				FunctionDenylist: config.DefaultTemplateFunctionDenylist,
				DisableSandbox:   false,
				ConsulRetry:      &config.RetryConfig{Backoff: pointer.Of(10 * time.Millisecond)},
			}},
		emitRate: DefaultMaxTemplateEventRate,
	}

	// Build the task environment
	a := mock.Alloc()
	task := a.Job.TaskGroups[0].Tasks[0]
	task.Name = TestTaskName
	harness.envBuilder = taskenv.NewBuilder(harness.node, a, task, region)
	harness.nomadNamespace = a.Namespace

	// Make a tempdir
	harness.taskDir = t.TempDir()
	harness.envBuilder.SetClientTaskRoot(harness.taskDir)

	if consul {
		var err error
		harness.consul, err = ctestutil.NewTestServerConfigT(t, func(c *ctestutil.TestServerConfig) {
			c.Peering = nil // fix for older versions of Consul (<1.13.0) that don't support peering
		})
		if err != nil {
			t.Fatalf("error starting test Consul server: %v", err)
		}
		harness.config.ConsulConfigs = map[string]*sconfig.ConsulConfig{
			structs.ConsulDefaultCluster: {
				Addr: harness.consul.HTTPAddr,
			}}
	}

	if vault {
		harness.vault = testutil.NewTestVault(t)
		harness.config.VaultConfigs = map[string]*sconfig.VaultConfig{
			structs.VaultDefaultCluster: harness.vault.Config,
		}
		harness.vaultToken = harness.vault.RootToken
	}

	return harness
}

func (h *testHarness) start(t *testing.T) {
	t.Helper()
	if err := h.startWithErr(); err != nil {
		t.Fatalf("failed to build task template manager: %v", err)
	}
}

func (h *testHarness) startWithErr() error {
	var err error
	h.manager, err = NewTaskTemplateManager(&TaskTemplateManagerConfig{
		UnblockCh:            h.mockHooks.UnblockCh,
		Lifecycle:            h.mockHooks,
		Events:               h.mockHooks,
		Templates:            h.templates,
		ClientConfig:         h.config,
		ConsulConfig:         h.config.GetDefaultConsul(),
		VaultToken:           h.vaultToken,
		VaultConfig:          h.config.GetDefaultVault(),
		TaskDir:              h.taskDir,
		EnvBuilder:           h.envBuilder,
		MaxTemplateEventRate: h.emitRate,
		TaskID:               uuid.Generate(),
	})
	return err
}

func (h *testHarness) setEmitRate(d time.Duration) {
	h.emitRate = d
}

// stop is used to stop any running Vault or Consul server plus the task manager
func (h *testHarness) stop() {
	if h.vault != nil {
		h.vault.Stop()
	}
	if h.consul != nil {
		h.consul.Stop()
	}
	if h.manager != nil {
		h.manager.Stop()
	}
	if h.taskDir != "" {
		os.RemoveAll(h.taskDir)
	}
}

func TestTaskTemplateManager_InvalidConfig(t *testing.T) {
	ci.Parallel(t)
	hooks := trtesting.NewMockTaskHooks()
	clientConfig := &config.Config{Region: "global"}
	taskDir := t.TempDir()
	a := mock.Alloc()
	envBuilder := taskenv.NewBuilder(mock.Node(), a, a.Job.TaskGroups[0].Tasks[0], clientConfig.Region)

	cases := []struct {
		name        string
		config      *TaskTemplateManagerConfig
		expectedErr string
	}{
		{
			name:        "nil config",
			config:      nil,
			expectedErr: "Nil config passed",
		},
		{
			name: "bad lifecycle hooks",
			config: &TaskTemplateManagerConfig{
				UnblockCh:            hooks.UnblockCh,
				Events:               hooks,
				ClientConfig:         clientConfig,
				TaskDir:              taskDir,
				EnvBuilder:           envBuilder,
				MaxTemplateEventRate: DefaultMaxTemplateEventRate,
				Logger:               testlog.HCLogger(t),
			},
			expectedErr: "lifecycle hooks",
		},
		{
			name: "bad event hooks",
			config: &TaskTemplateManagerConfig{
				UnblockCh:            hooks.UnblockCh,
				Lifecycle:            hooks,
				ClientConfig:         clientConfig,
				TaskDir:              taskDir,
				EnvBuilder:           envBuilder,
				MaxTemplateEventRate: DefaultMaxTemplateEventRate,
			},
			expectedErr: "event hook",
		},
		{
			name: "bad client config",
			config: &TaskTemplateManagerConfig{
				UnblockCh:            hooks.UnblockCh,
				Lifecycle:            hooks,
				Events:               hooks,
				TaskDir:              taskDir,
				EnvBuilder:           envBuilder,
				MaxTemplateEventRate: DefaultMaxTemplateEventRate,
			},
			expectedErr: "client config",
		},
		{
			name: "bad task dir",
			config: &TaskTemplateManagerConfig{
				UnblockCh:            hooks.UnblockCh,
				ClientConfig:         clientConfig,
				Lifecycle:            hooks,
				Events:               hooks,
				EnvBuilder:           envBuilder,
				MaxTemplateEventRate: DefaultMaxTemplateEventRate,
			},
			expectedErr: "task directory",
		},
		{
			name: "bad env builder",
			config: &TaskTemplateManagerConfig{
				UnblockCh:            hooks.UnblockCh,
				ClientConfig:         clientConfig,
				Lifecycle:            hooks,
				Events:               hooks,
				TaskDir:              taskDir,
				MaxTemplateEventRate: DefaultMaxTemplateEventRate,
			},
			expectedErr: "task environment",
		},
		{
			name: "bad max event rate",
			config: &TaskTemplateManagerConfig{
				UnblockCh:    hooks.UnblockCh,
				ClientConfig: clientConfig,
				Lifecycle:    hooks,
				Events:       hooks,
				TaskDir:      taskDir,
				EnvBuilder:   envBuilder,
			},
			expectedErr: "template event rate",
		},
		{
			name: "valid",
			config: &TaskTemplateManagerConfig{
				UnblockCh:            hooks.UnblockCh,
				ClientConfig:         clientConfig,
				Lifecycle:            hooks,
				Events:               hooks,
				TaskDir:              taskDir,
				EnvBuilder:           envBuilder,
				MaxTemplateEventRate: DefaultMaxTemplateEventRate,
			},
		},
		{
			name: "invalid signal",
			config: &TaskTemplateManagerConfig{
				UnblockCh: hooks.UnblockCh,
				Templates: []*structs.Template{
					{
						DestPath:     "foo",
						EmbeddedTmpl: "hello, world",
						ChangeMode:   structs.TemplateChangeModeSignal,
						ChangeSignal: "foobarbaz",
					},
				},
				ClientConfig:         clientConfig,
				Lifecycle:            hooks,
				Events:               hooks,
				TaskDir:              taskDir,
				EnvBuilder:           envBuilder,
				MaxTemplateEventRate: DefaultMaxTemplateEventRate,
			},
			expectedErr: "parse signal",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.config != nil {
				c.config.TaskID = c.name
				c.config.Logger = testlog.HCLogger(t)
			}
			_, err := NewTaskTemplateManager(c.config)
			if err != nil {
				if c.expectedErr == "" {
					t.Fatalf("unexpected error: %v", err)
				} else if !strings.Contains(err.Error(), c.expectedErr) {
					t.Fatalf("expected error to contain %q; got %q", c.expectedErr, err.Error())
				}
			} else if c.expectedErr != "" {
				t.Fatalf("expected an error to contain %q", c.expectedErr)
			}
		})
	}
}

func TestTaskTemplateManager_HostPath(t *testing.T) {
	ci.Parallel(t)
	// Make a template that will render immediately and write it to a tmp file
	f, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatalf("Bad: %v", err)
	}
	defer f.Close()
	defer os.Remove(f.Name())

	content := "hello, world!"
	if _, err := io.WriteString(f, content); err != nil {
		t.Fatalf("Bad: %v", err)
	}

	file := "my.tmpl"
	template := &structs.Template{
		SourcePath: f.Name(),
		DestPath:   file,
		ChangeMode: structs.TemplateChangeModeNoop,
	}

	harness := newTestHarness(t, []*structs.Template{template}, false, false)
	harness.config.TemplateConfig.DisableSandbox = true
	err = harness.startWithErr()
	if err != nil {
		t.Fatalf("couldn't setup initial harness: %v", err)
	}
	defer harness.stop()

	// Wait for the unblock
	select {
	case <-harness.mockHooks.UnblockCh:
	case <-time.After(time.Duration(5*testutil.TestMultiplier()) * time.Second):
		t.Fatalf("Task unblock should have been called")
	}

	// Check the file is there
	path := filepath.Join(harness.taskDir, file)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read rendered template from %q: %v", path, err)
	}

	if s := string(raw); s != content {
		t.Fatalf("Unexpected template data; got %q, want %q", s, content)
	}

	// Change the config to disallow host sources
	harness = newTestHarness(t, []*structs.Template{template}, false, false)
	err = harness.startWithErr()
	if err == nil || !strings.Contains(err.Error(), "escapes alloc directory") {
		t.Fatalf("Expected absolute template path disallowed for %q: %v",
			template.SourcePath, err)
	}

	template.SourcePath = "../../../../../../" + file
	harness = newTestHarness(t, []*structs.Template{template}, false, false)
	err = harness.startWithErr()
	if err == nil || !strings.Contains(err.Error(), "escapes alloc directory") {
		t.Fatalf("Expected directory traversal out of %q disallowed for %q: %v",
			harness.taskDir, template.SourcePath, err)
	}

	// Build a new task environment
	a := mock.Alloc()
	task := a.Job.TaskGroups[0].Tasks[0]
	task.Name = TestTaskName
	task.Meta = map[string]string{"ESCAPE": "../"}

	template.SourcePath = "${NOMAD_META_ESCAPE}${NOMAD_META_ESCAPE}${NOMAD_META_ESCAPE}${NOMAD_META_ESCAPE}${NOMAD_META_ESCAPE}${NOMAD_META_ESCAPE}" + file
	harness = newTestHarness(t, []*structs.Template{template}, false, false)
	harness.envBuilder = taskenv.NewBuilder(harness.node, a, task, "global")
	err = harness.startWithErr()
	if err == nil || !strings.Contains(err.Error(), "escapes alloc directory") {
		t.Fatalf("Expected directory traversal out of %q via interpolation disallowed for %q: %v",
			harness.taskDir, template.SourcePath, err)
	}

	// Test with destination too
	template.SourcePath = f.Name()
	template.DestPath = "../../../../../../" + file
	harness = newTestHarness(t, []*structs.Template{template}, false, false)
	harness.envBuilder = taskenv.NewBuilder(harness.node, a, task, "global")
	err = harness.startWithErr()
	must.ErrorContains(t, err, "escapes alloc directory", must.Sprintf(
		"Expected directory traversal out of %q via interpolation disallowed for %q: %v",
		harness.taskDir, template.SourcePath, err))
}

func TestTaskTemplateManager_Unblock_Static(t *testing.T) {
	ci.Parallel(t)
	// Make a template that will render immediately
	content := "hello, world!"
	file := "my.tmpl"
	template := &structs.Template{
		EmbeddedTmpl: content,
		DestPath:     file,
		ChangeMode:   structs.TemplateChangeModeNoop,
	}

	a := mock.Alloc()
	task := a.Job.TaskGroups[0].Tasks[0]
	task.Name = TestTaskName

	harness := newTestHarness(t, []*structs.Template{template}, false, false)
	harness.envBuilder = taskenv.NewBuilder(harness.node, a, task, "global")
	harness.envBuilder.SetClientTaskRoot(harness.taskDir)
	harness.start(t)
	defer harness.stop()

	// Wait for the unblock
	select {
	case <-harness.mockHooks.UnblockCh:
	case <-time.After(time.Duration(5*testutil.TestMultiplier()) * time.Second):
		t.Fatalf("Task unblock should have been called")
	}

	// Check the file is there
	path := filepath.Join(harness.taskDir, file)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read rendered template from %q: %v", path, err)
	}

	if s := string(raw); s != content {
		t.Fatalf("Unexpected template data; got %q, want %q", s, content)
	}
}

func TestTaskTemplateManager_Unblock_Static_NomadEnv(t *testing.T) {
	ci.Parallel(t)
	// Make a template that will render immediately
	content := `Hello Nomad Task: {{env "NOMAD_TASK_NAME"}}`
	expected := fmt.Sprintf("Hello Nomad Task: %s", TestTaskName)
	file := "my.tmpl"
	template := &structs.Template{
		EmbeddedTmpl: content,
		DestPath:     file,
		ChangeMode:   structs.TemplateChangeModeNoop,
	}

	a := mock.Alloc()
	task := a.Job.TaskGroups[0].Tasks[0]
	task.Name = TestTaskName

	harness := newTestHarness(t, []*structs.Template{template}, false, false)
	harness.envBuilder = taskenv.NewBuilder(harness.node, a, task, "global")
	harness.envBuilder.SetClientTaskRoot(harness.taskDir)
	harness.start(t)
	defer harness.stop()

	// Wait for the unblock
	select {
	case <-harness.mockHooks.UnblockCh:
	case <-time.After(time.Duration(5*testutil.TestMultiplier()) * time.Second):
		t.Fatalf("Task unblock should have been called")
	}

	// Check the file is there
	path := filepath.Join(harness.taskDir, file)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read rendered template from %q: %v", path, err)
	}

	if s := string(raw); s != expected {
		t.Fatalf("Unexpected template data; got %q, want %q", s, expected)
	}
}

func TestTaskTemplateManager_Unblock_Static_AlreadyRendered(t *testing.T) {
	ci.Parallel(t)
	// Make a template that will render immediately
	content := "hello, world!"
	file := "my.tmpl"
	template := &structs.Template{
		EmbeddedTmpl: content,
		DestPath:     file,
		ChangeMode:   structs.TemplateChangeModeNoop,
	}

	a := mock.Alloc()
	task := a.Job.TaskGroups[0].Tasks[0]
	task.Name = TestTaskName

	harness := newTestHarness(t, []*structs.Template{template}, false, false)
	harness.envBuilder = taskenv.NewBuilder(harness.node, a, task, "global")
	harness.envBuilder.SetClientTaskRoot(harness.taskDir)

	// Write the contents
	path := filepath.Join(harness.taskDir, file)
	must.NoError(t, os.WriteFile(path, []byte(content), 0777))

	harness.start(t)
	defer harness.stop()

	// Wait for the unblock
	select {
	case <-harness.mockHooks.UnblockCh:
	case <-time.After(time.Duration(5*testutil.TestMultiplier()) * time.Second):
		t.Fatalf("Task unblock should have been called")
	}

	// Check the file is there
	path = filepath.Join(harness.taskDir, file)
	raw, err := os.ReadFile(path)
	must.NoError(t, err, must.Sprintf(
		"Failed to read rendered template from %q", path))

	must.Eq(t, content, string(raw), must.Sprint("Unexpected template data"))
}

func TestTaskTemplateManager_Unblock_Consul(t *testing.T) {
	ci.Parallel(t)
	clienttestutil.RequireConsul(t)

	// Make a template that will render based on a key in Consul
	key := "foo"
	content := "barbaz"
	embedded := fmt.Sprintf(`{{key "%s"}}`, key)
	file := "my.tmpl"
	template := &structs.Template{
		EmbeddedTmpl: embedded,
		DestPath:     file,
		ChangeMode:   structs.TemplateChangeModeNoop,
	}

	harness := newTestHarness(t, []*structs.Template{template}, true, false)
	harness.start(t)
	defer harness.stop()

	// Ensure no unblock
	select {
	case <-harness.mockHooks.UnblockCh:
		t.Fatalf("Task unblock should have not have been called")
	case <-time.After(time.Duration(1*testutil.TestMultiplier()) * time.Second):
	}

	// Write the key to Consul
	harness.consul.SetKV(t, key, []byte(content))

	// Wait for the unblock
	select {
	case <-harness.mockHooks.UnblockCh:
	case <-time.After(time.Duration(5*testutil.TestMultiplier()) * time.Second):
		t.Fatalf("Task unblock should have been called")
	}

	// Check the file is there
	path := filepath.Join(harness.taskDir, file)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read rendered template from %q: %v", path, err)
	}

	if s := string(raw); s != content {
		t.Fatalf("Unexpected template data; got %q, want %q", s, content)
	}
}

func TestTaskTemplateManager_Unblock_Vault(t *testing.T) {
	ci.Parallel(t)
	clienttestutil.RequireVault(t)

	// Make a template that will render based on a key in Vault
	vaultPath := "secret/data/password"
	key := "password"
	content := "barbaz"
	embedded := fmt.Sprintf(`{{with secret "%s"}}{{.Data.data.%s}}{{end}}`, vaultPath, key)
	file := "my.tmpl"
	template := &structs.Template{
		EmbeddedTmpl: embedded,
		DestPath:     file,
		ChangeMode:   structs.TemplateChangeModeNoop,
	}

	harness := newTestHarness(t, []*structs.Template{template}, false, true)
	harness.start(t)
	defer harness.stop()

	// Ensure no unblock
	select {
	case <-harness.mockHooks.UnblockCh:
		t.Fatalf("Task unblock should not have been called")
	case <-time.After(time.Duration(1*testutil.TestMultiplier()) * time.Second):
	}

	// Write the secret to Vault
	logical := harness.vault.Client.Logical()
	_, err := logical.Write(vaultPath, map[string]interface{}{"data": map[string]interface{}{key: content}})
	must.NoError(t, err)

	// Wait for the unblock
	select {
	case <-harness.mockHooks.UnblockCh:
	case <-time.After(time.Duration(5*testutil.TestMultiplier()) * time.Second):
		t.Fatalf("Task unblock should have been called")
	}

	// Check the file is there
	path := filepath.Join(harness.taskDir, file)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read rendered template from %q: %v", path, err)
	}

	if s := string(raw); s != content {
		t.Fatalf("Unexpected template data; got %q, want %q", s, content)
	}
}

func TestTaskTemplateManager_Unblock_Multi_Template(t *testing.T) {
	ci.Parallel(t)
	clienttestutil.RequireConsul(t)

	// Make a template that will render immediately
	staticContent := "hello, world!"
	staticFile := "my.tmpl"
	template := &structs.Template{
		EmbeddedTmpl: staticContent,
		DestPath:     staticFile,
		ChangeMode:   structs.TemplateChangeModeNoop,
	}

	// Make a template that will render based on a key in Consul
	consulKey := "foo"
	consulContent := "barbaz"
	consulEmbedded := fmt.Sprintf(`{{key "%s"}}`, consulKey)
	consulFile := "consul.tmpl"
	template2 := &structs.Template{
		EmbeddedTmpl: consulEmbedded,
		DestPath:     consulFile,
		ChangeMode:   structs.TemplateChangeModeNoop,
	}

	harness := newTestHarness(t, []*structs.Template{template, template2}, true, false)
	harness.start(t)
	defer harness.stop()

	// Ensure no unblock
	select {
	case <-harness.mockHooks.UnblockCh:
		t.Fatalf("Task unblock should have not have been called")
	case <-time.After(time.Duration(1*testutil.TestMultiplier()) * time.Second):
	}

	// Check that the static file has been rendered
	path := filepath.Join(harness.taskDir, staticFile)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read rendered template from %q: %v", path, err)
	}

	if s := string(raw); s != staticContent {
		t.Fatalf("Unexpected template data; got %q, want %q", s, staticContent)
	}

	// Write the key to Consul
	harness.consul.SetKV(t, consulKey, []byte(consulContent))

	// Wait for the unblock
	select {
	case <-harness.mockHooks.UnblockCh:
	case <-time.After(time.Duration(5*testutil.TestMultiplier()) * time.Second):
		t.Fatalf("Task unblock should have been called")
	}

	// Check the consul file is there
	path = filepath.Join(harness.taskDir, consulFile)
	raw, err = os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read rendered template from %q: %v", path, err)
	}

	if s := string(raw); s != consulContent {
		t.Fatalf("Unexpected template data; got %q, want %q", s, consulContent)
	}
}

// TestTaskTemplateManager_FirstRender_Restored tests that a task that's been
// restored renders and triggers its change mode if the template has changed
func TestTaskTemplateManager_FirstRender_Restored(t *testing.T) {
	ci.Parallel(t)
	clienttestutil.RequireVault(t)

	// Make a template that will render based on a key in Vault
	vaultPath := "secret/data/password"
	key := "password"
	content := "barbaz"
	embedded := fmt.Sprintf(`{{with secret "%s"}}{{.Data.data.%s}}{{end}}`, vaultPath, key)
	file := "my.tmpl"
	template := &structs.Template{
		EmbeddedTmpl: embedded,
		DestPath:     file,
		ChangeMode:   structs.TemplateChangeModeRestart,
	}

	harness := newTestHarness(t, []*structs.Template{template}, false, true)
	harness.start(t)
	defer harness.stop()

	// Ensure no unblock
	select {
	case <-harness.mockHooks.UnblockCh:
		t.Fatal("Task unblock should not have been called")
	case <-time.After(time.Duration(1*testutil.TestMultiplier()) * time.Second):
	}

	// Write the secret to Vault
	logical := harness.vault.Client.Logical()
	_, err := logical.Write(vaultPath, map[string]interface{}{"data": map[string]interface{}{key: content}})
	must.NoError(t, err)

	// Wait for the unblock
	select {
	case <-harness.mockHooks.UnblockCh:
	case <-time.After(time.Duration(5*testutil.TestMultiplier()) * time.Second):
		t.Fatal("Task unblock should have been called")
	}

	// Check the file is there
	path := filepath.Join(harness.taskDir, file)
	raw, err := os.ReadFile(path)
	must.NoError(t, err, must.Sprintf("Failed to read rendered template from %q", path))
	must.Eq(t, content, string(raw), must.Sprintf("Unexpected template data; got %s, want %q", raw, content))

	// task is now running
	harness.mockHooks.HasHandle = true

	// simulate a client restart
	harness.manager.Stop()
	harness.mockHooks.UnblockCh = make(chan struct{}, 1)
	harness.start(t)

	// Wait for the unblock
	select {
	case <-harness.mockHooks.UnblockCh:
	case <-time.After(time.Duration(5*testutil.TestMultiplier()) * time.Second):
		t.Fatal("Task unblock should have been called")
	}

	select {
	case <-harness.mockHooks.RestartCh:
		t.Fatal("should not have restarted", harness.mockHooks)
	case <-harness.mockHooks.SignalCh:
		t.Fatal("should not have received signal", harness.mockHooks)
	case <-time.After(time.Duration(1*testutil.TestMultiplier()) * time.Second):
	}

	// simulate a client restart and TTL expiry
	harness.manager.Stop()
	content = "bazbar"
	_, err = logical.Write(vaultPath, map[string]interface{}{"data": map[string]interface{}{key: content}})
	must.NoError(t, err)
	harness.mockHooks.UnblockCh = make(chan struct{}, 1)
	harness.start(t)

	// Wait for the unblock
	select {
	case <-harness.mockHooks.UnblockCh:
	case <-time.After(time.Duration(5*testutil.TestMultiplier()) * time.Second):
		t.Fatal("Task unblock should have been called")
	}

	// Wait for restart
	timeout := time.After(time.Duration(1*testutil.TestMultiplier()) * time.Second)
OUTER:
	for {
		select {
		case <-harness.mockHooks.RestartCh:
			break OUTER
		case <-harness.mockHooks.SignalCh:
			t.Fatal("Signal with restart policy", harness.mockHooks)
		case <-timeout:
			t.Fatal("Should have received a restart", harness.mockHooks)
		}
	}
}

func TestTaskTemplateManager_Rerender_Noop(t *testing.T) {
	ci.Parallel(t)
	clienttestutil.RequireConsul(t)

	// Make a template that will render based on a key in Consul
	key := "foo"
	content1 := "bar"
	content2 := "baz"
	embedded := fmt.Sprintf(`{{key "%s"}}`, key)
	file := "my.tmpl"
	template := &structs.Template{
		EmbeddedTmpl: embedded,
		DestPath:     file,
		ChangeMode:   structs.TemplateChangeModeNoop,
	}

	harness := newTestHarness(t, []*structs.Template{template}, true, false)
	harness.start(t)
	defer harness.stop()

	// Ensure no unblock
	select {
	case <-harness.mockHooks.UnblockCh:
		t.Fatalf("Task unblock should have not have been called")
	case <-time.After(time.Duration(1*testutil.TestMultiplier()) * time.Second):
	}

	// Write the key to Consul
	harness.consul.SetKV(t, key, []byte(content1))

	// Wait for the unblock
	select {
	case <-harness.mockHooks.UnblockCh:
	case <-time.After(time.Duration(5*testutil.TestMultiplier()) * time.Second):
		t.Fatalf("Task unblock should have been called")
	}

	// Check the file is there
	path := filepath.Join(harness.taskDir, file)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read rendered template from %q: %v", path, err)
	}

	if s := string(raw); s != content1 {
		t.Fatalf("Unexpected template data; got %q, want %q", s, content1)
	}

	// Update the key in Consul
	harness.consul.SetKV(t, key, []byte(content2))

	select {
	case <-harness.mockHooks.RestartCh:
		t.Fatalf("Noop ignored: %+v", harness.mockHooks)
	case <-harness.mockHooks.SignalCh:
		t.Fatalf("Noop ignored: %+v", harness.mockHooks)
	case <-time.After(time.Duration(1*testutil.TestMultiplier()) * time.Second):
	}

	// Check the file has been updated
	path = filepath.Join(harness.taskDir, file)
	raw, err = os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read rendered template from %q: %v", path, err)
	}

	if s := string(raw); s != content2 {
		t.Fatalf("Unexpected template data; got %q, want %q", s, content2)
	}
}

func TestTaskTemplateManager_Rerender_Signal(t *testing.T) {
	ci.Parallel(t)
	clienttestutil.RequireConsul(t)

	// Make a template that renders based on a key in Consul and sends SIGALRM
	key1 := "foo"
	content1_1 := "bar"
	content1_2 := "baz"
	embedded1 := fmt.Sprintf(`{{key "%s"}}`, key1)
	file1 := "my.tmpl"
	template := &structs.Template{
		EmbeddedTmpl: embedded1,
		DestPath:     file1,
		ChangeMode:   structs.TemplateChangeModeSignal,
		ChangeSignal: "SIGALRM",
	}

	// Make a template that renders based on a key in Consul and sends SIGBUS
	key2 := "bam"
	content2_1 := "cat"
	content2_2 := "dog"
	embedded2 := fmt.Sprintf(`{{key "%s"}}`, key2)
	file2 := "my-second.tmpl"
	template2 := &structs.Template{
		EmbeddedTmpl: embedded2,
		DestPath:     file2,
		ChangeMode:   structs.TemplateChangeModeSignal,
		ChangeSignal: "SIGBUS",
	}

	harness := newTestHarness(t, []*structs.Template{template, template2}, true, false)
	harness.start(t)
	defer harness.stop()

	// Ensure no unblock
	select {
	case <-harness.mockHooks.UnblockCh:
		t.Fatalf("Task unblock should have not have been called")
	case <-time.After(time.Duration(1*testutil.TestMultiplier()) * time.Second):
	}

	// Write the key to Consul
	harness.consul.SetKV(t, key1, []byte(content1_1))
	harness.consul.SetKV(t, key2, []byte(content2_1))

	// Wait for the unblock
	select {
	case <-harness.mockHooks.UnblockCh:
	case <-time.After(time.Duration(5*testutil.TestMultiplier()) * time.Second):
		t.Fatalf("Task unblock should have been called")
	}

	if len(harness.mockHooks.Signals()) != 0 {
		t.Fatalf("Should not have received any signals: %+v", harness.mockHooks)
	}

	// Update the keys in Consul
	harness.consul.SetKV(t, key1, []byte(content1_2))
	harness.consul.SetKV(t, key2, []byte(content2_2))

	// Wait for signals
	timeout := time.After(time.Duration(1*testutil.TestMultiplier()) * time.Second)
OUTER:
	for {
		select {
		case <-harness.mockHooks.RestartCh:
			t.Fatalf("Restart with signal policy: %+v", harness.mockHooks)
		case <-harness.mockHooks.SignalCh:
			s := harness.mockHooks.Signals()
			if len(s) != 2 {
				continue
			}
			break OUTER
		case <-timeout:
			t.Fatalf("Should have received two signals: %+v", harness.mockHooks)
		}
	}

	// Check the files have  been updated
	path := filepath.Join(harness.taskDir, file1)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read rendered template from %q: %v", path, err)
	}

	if s := string(raw); s != content1_2 {
		t.Fatalf("Unexpected template data; got %q, want %q", s, content1_2)
	}

	path = filepath.Join(harness.taskDir, file2)
	raw, err = os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read rendered template from %q: %v", path, err)
	}

	if s := string(raw); s != content2_2 {
		t.Fatalf("Unexpected template data; got %q, want %q", s, content2_2)
	}
}

func TestTaskTemplateManager_Rerender_Restart(t *testing.T) {
	ci.Parallel(t)
	clienttestutil.RequireConsul(t)

	// Make a template that renders based on a key in Consul and sends restart
	key1 := "bam"
	content1_1 := "cat"
	content1_2 := "dog"
	embedded1 := fmt.Sprintf(`{{key "%s"}}`, key1)
	file1 := "my.tmpl"
	template := &structs.Template{
		EmbeddedTmpl: embedded1,
		DestPath:     file1,
		ChangeMode:   structs.TemplateChangeModeRestart,
	}

	harness := newTestHarness(t, []*structs.Template{template}, true, false)
	harness.start(t)
	defer harness.stop()

	// Ensure no unblock
	select {
	case <-harness.mockHooks.UnblockCh:
		t.Fatalf("Task unblock should have not have been called")
	case <-time.After(time.Duration(1*testutil.TestMultiplier()) * time.Second):
	}

	// Write the key to Consul
	harness.consul.SetKV(t, key1, []byte(content1_1))

	// Wait for the unblock
	select {
	case <-harness.mockHooks.UnblockCh:
	case <-time.After(time.Duration(5*testutil.TestMultiplier()) * time.Second):
		t.Fatalf("Task unblock should have been called")
	}

	// Update the keys in Consul
	harness.consul.SetKV(t, key1, []byte(content1_2))

	// Wait for restart
	timeout := time.After(time.Duration(1*testutil.TestMultiplier()) * time.Second)
OUTER:
	for {
		select {
		case <-harness.mockHooks.RestartCh:
			break OUTER
		case <-harness.mockHooks.SignalCh:
			t.Fatalf("Signal with restart policy: %+v", harness.mockHooks)
		case <-timeout:
			t.Fatalf("Should have received a restart: %+v", harness.mockHooks)
		}
	}

	// Check the files have  been updated
	path := filepath.Join(harness.taskDir, file1)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read rendered template from %q: %v", path, err)
	}

	if s := string(raw); s != content1_2 {
		t.Fatalf("Unexpected template data; got %q, want %q", s, content1_2)
	}
}

func TestTaskTemplateManager_Interpolate_Destination(t *testing.T) {
	ci.Parallel(t)
	// Make a template that will have its destination interpolated
	content := "hello, world!"
	file := "${node.unique.id}.tmpl"
	template := &structs.Template{
		EmbeddedTmpl: content,
		DestPath:     file,
		ChangeMode:   structs.TemplateChangeModeNoop,
	}

	harness := newTestHarness(t, []*structs.Template{template}, false, false)
	harness.config.TemplateConfig.DisableSandbox = true // no real alloc in this test
	harness.start(t)
	defer harness.stop()

	// Ensure unblock
	select {
	case <-harness.mockHooks.UnblockCh:
	case <-time.After(time.Duration(5*testutil.TestMultiplier()) * time.Second):
		t.Fatalf("Task unblock should have been called")
	}

	// Check the file is there
	actual := fmt.Sprintf("%s.tmpl", harness.node.ID)
	path := filepath.Join(harness.taskDir, actual)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read rendered template from %q: %v", path, err)
	}

	if s := string(raw); s != content {
		t.Fatalf("Unexpected template data; got %q, want %q", s, content)
	}
}

func TestTaskTemplateManager_Signal_Error(t *testing.T) {
	ci.Parallel(t)
	clienttestutil.RequireConsul(t)

	// Make a template that renders based on a key in Consul and sends SIGALRM
	key1 := "foo"
	content1 := "bar"
	content2 := "baz"
	embedded1 := fmt.Sprintf(`{{key "%s"}}`, key1)
	file1 := "my.tmpl"
	template := &structs.Template{
		EmbeddedTmpl: embedded1,
		DestPath:     file1,
		ChangeMode:   structs.TemplateChangeModeSignal,
		ChangeSignal: "SIGALRM",
	}

	harness := newTestHarness(t, []*structs.Template{template}, true, false)
	harness.start(t)
	defer harness.stop()

	harness.mockHooks.SignalError = fmt.Errorf("test error")

	// Write the key to Consul
	harness.consul.SetKV(t, key1, []byte(content1))

	// Wait a little
	select {
	case <-harness.mockHooks.UnblockCh:
	case <-time.After(time.Duration(2*testutil.TestMultiplier()) * time.Second):
		t.Fatalf("Should have received unblock: %+v", harness.mockHooks)
	}

	// Write the key to Consul
	harness.consul.SetKV(t, key1, []byte(content2))

	// Wait for kill channel
	select {
	case <-harness.mockHooks.KillCh:
		break
	case <-time.After(time.Duration(1*testutil.TestMultiplier()) * time.Second):
		t.Fatalf("Should have received a signals: %+v", harness.mockHooks)
	}

	must.NotNil(t, harness.mockHooks.KillEvent)
	must.StrContains(t, harness.mockHooks.KillEvent().DisplayMessage, "failed to send signals")
}

func TestTaskTemplateManager_ScriptExecution(t *testing.T) {
	ci.Parallel(t)
	clienttestutil.RequireConsul(t)

	// Make a template that renders based on a key in Consul and triggers script
	key1 := "bam"
	key2 := "bar"
	content1_1 := "cat"
	content1_2 := "dog"

	t1 := &structs.Template{
		EmbeddedTmpl: `
FOO={{key "bam"}}
`,
		DestPath:   "test.env",
		ChangeMode: structs.TemplateChangeModeScript,
		ChangeScript: &structs.ChangeScript{
			Command:     "/bin/foo",
			Args:        []string{},
			Timeout:     5 * time.Second,
			FailOnError: false,
		},
		Envvars: true,
	}
	t2 := &structs.Template{
		EmbeddedTmpl: `
BAR={{key "bar"}}
`,
		DestPath:   "test2.env",
		ChangeMode: structs.TemplateChangeModeScript,
		ChangeScript: &structs.ChangeScript{
			Command:     "/bin/foo",
			Args:        []string{},
			Timeout:     5 * time.Second,
			FailOnError: false,
		},
		Envvars: true,
	}

	harness := newTestHarness(t, []*structs.Template{t1, t2}, true, false)
	harness.mockHooks.SetupExecTest(0, nil)
	harness.start(t)
	defer harness.stop()

	// Ensure no unblock
	select {
	case <-harness.mockHooks.UnblockCh:
		t.Fatal(t, "Task unblock should not have been called")
	case <-time.After(time.Duration(1*testutil.TestMultiplier()) * time.Second):
	}

	// Write the key to Consul
	harness.consul.SetKV(t, key1, []byte(content1_1))
	harness.consul.SetKV(t, key2, []byte(content1_1))

	// Wait for the unblock
	select {
	case <-harness.mockHooks.UnblockCh:
	case <-time.After(time.Duration(5*testutil.TestMultiplier()) * time.Second):
		t.Fatal(t, "Task unblock should have been called")
	}

	// Update the keys in Consul
	harness.consul.SetKV(t, key1, []byte(content1_2))

	// Wait for script execution
	timeout := time.After(time.Duration(5*testutil.TestMultiplier()) * time.Second)
OUTER:
	for {
		select {
		case <-harness.mockHooks.RestartCh:
			t.Fatal(t, "restart not expected")
		case ev := <-harness.mockHooks.EmitEventCh:
			if strings.Contains(ev.DisplayMessage, t1.ChangeScript.Command) {
				break OUTER
			}
		case <-harness.mockHooks.SignalCh:
			t.Fatal(t, "signal not expected")
		case <-timeout:
			t.Fatal(t, "should have received an event")
		}
	}
}

// TestTaskTemplateManager_ScriptExecutionFailTask tests whether we fail the
// task upon script execution failure if that's how it's configured.
func TestTaskTemplateManager_ScriptExecutionFailTask(t *testing.T) {
	ci.Parallel(t)
	clienttestutil.RequireConsul(t)

	// Make a template that renders based on a key in Consul and triggers script
	key1 := "bam"
	key2 := "bar"
	content1_1 := "cat"
	content1_2 := "dog"
	t1 := &structs.Template{
		EmbeddedTmpl: `
FOO={{key "bam"}}
`,
		DestPath:   "test.env",
		ChangeMode: structs.TemplateChangeModeScript,
		ChangeScript: &structs.ChangeScript{
			Command:     "/bin/foo",
			Args:        []string{},
			Timeout:     5 * time.Second,
			FailOnError: true,
		},
		Envvars: true,
	}
	t2 := &structs.Template{
		EmbeddedTmpl: `
BAR={{key "bar"}}
`,
		DestPath:   "test2.env",
		ChangeMode: structs.TemplateChangeModeScript,
		ChangeScript: &structs.ChangeScript{
			Command:     "/bin/foo",
			Args:        []string{},
			Timeout:     5 * time.Second,
			FailOnError: false,
		},
		Envvars: true,
	}

	harness := newTestHarness(t, []*structs.Template{t1, t2}, true, false)
	harness.mockHooks.SetupExecTest(1, fmt.Errorf("Script failed"))
	harness.start(t)
	defer harness.stop()

	// Ensure no unblock
	select {
	case <-harness.mockHooks.UnblockCh:
		t.Fatal("Task unblock should not have been called")
	case <-time.After(time.Duration(1*testutil.TestMultiplier()) * time.Second):
	}

	// Write the key to Consul
	harness.consul.SetKV(t, key1, []byte(content1_1))
	harness.consul.SetKV(t, key2, []byte(content1_1))

	// Wait for the unblock
	select {
	case <-harness.mockHooks.UnblockCh:
	case <-time.After(time.Duration(5*testutil.TestMultiplier()) * time.Second):
		t.Fatal("Task unblock should have been called")
	}

	// Update the keys in Consul
	harness.consul.SetKV(t, key1, []byte(content1_2))

	// Wait for kill channel
	select {
	case <-harness.mockHooks.KillCh:
		break
	case <-time.After(time.Duration(1*testutil.TestMultiplier()) * time.Second):
		t.Fatalf("Should have received a signals: %+v", harness.mockHooks)
	}

	must.NotNil(t, harness.mockHooks.KillEvent)
	must.StrContains(t, harness.mockHooks.KillEvent().DisplayMessage, "task is being killed")
}

func TestTaskTemplateManager_ChangeModeMixed(t *testing.T) {
	ci.Parallel(t)
	clienttestutil.RequireConsul(t)

	templateRestart := &structs.Template{
		EmbeddedTmpl: `
RESTART={{key "restart"}}
COMMON={{key "common"}}
`,
		DestPath:   "restart",
		ChangeMode: structs.TemplateChangeModeRestart,
	}
	templateSignal := &structs.Template{
		EmbeddedTmpl: `
SIGNAL={{key "signal"}}
COMMON={{key "common"}}
`,
		DestPath:     "signal",
		ChangeMode:   structs.TemplateChangeModeSignal,
		ChangeSignal: "SIGALRM",
	}
	templateScript := &structs.Template{
		EmbeddedTmpl: `
SCRIPT={{key "script"}}
COMMON={{key "common"}}
`,
		DestPath:   "script",
		ChangeMode: structs.TemplateChangeModeScript,
		ChangeScript: &structs.ChangeScript{
			Command:     "/bin/foo",
			Args:        []string{},
			Timeout:     5 * time.Second,
			FailOnError: true,
		},
	}
	templates := []*structs.Template{
		templateRestart,
		templateSignal,
		templateScript,
	}

	harness := newTestHarness(t, templates, true, false)
	harness.mockHooks.SetupExecTest(0, nil)
	harness.start(t)
	defer harness.stop()

	// Ensure no unblock
	select {
	case <-harness.mockHooks.UnblockCh:
		t.Fatal(t, "Task unblock should not have been called")
	case <-time.After(time.Duration(1*testutil.TestMultiplier()) * time.Second):
	}

	// Write the key to Consul
	harness.consul.SetKV(t, "common", []byte(fmt.Sprintf("%v", time.Now())))
	harness.consul.SetKV(t, "restart", []byte(fmt.Sprintf("%v", time.Now())))
	harness.consul.SetKV(t, "signal", []byte(fmt.Sprintf("%v", time.Now())))
	harness.consul.SetKV(t, "script", []byte(fmt.Sprintf("%v", time.Now())))

	// Wait for the unblock
	select {
	case <-harness.mockHooks.UnblockCh:
	case <-time.After(time.Duration(5*testutil.TestMultiplier()) * time.Second):
		t.Fatal(t, "Task unblock should have been called")
	}

	t.Run("restart takes precedence", func(t *testing.T) {
		// Update the common Consul key.
		harness.consul.SetKV(t, "common", []byte(fmt.Sprintf("%v", time.Now())))

		// Collect some events.
		timeout := time.After(time.Duration(3*testutil.TestMultiplier()) * time.Second)
		events := []*structs.TaskEvent{}
	OUTER:
		for {
			select {
			case <-harness.mockHooks.RestartCh:
				// Consume restarts so the channel is clean for other tests.
			case <-harness.mockHooks.SignalCh:
				t.Fatal(t, "signal not expected")
			case ev := <-harness.mockHooks.EmitEventCh:
				events = append(events, ev)
			case <-timeout:
				break OUTER
			}
		}

		for _, ev := range events {
			must.StrNotContains(t, ev.DisplayMessage, templateScript.ChangeScript.Command)
			must.StrNotContains(t, ev.Type, structs.TaskSignaling)
		}
	})

	t.Run("signal and script", func(t *testing.T) {
		// Update the signal and script Consul keys.
		harness.consul.SetKV(t, "signal", []byte(fmt.Sprintf("%v", time.Now())))
		harness.consul.SetKV(t, "script", []byte(fmt.Sprintf("%v", time.Now())))

		// Wait for a events.
		var gotSignal, gotScript bool
		timeout := time.After(time.Duration(5*testutil.TestMultiplier()) * time.Second)
		for {
			select {
			case <-harness.mockHooks.RestartCh:
				t.Fatal(t, "restart not expected")
			case ev := <-harness.mockHooks.EmitEventCh:
				if strings.Contains(ev.DisplayMessage, templateScript.ChangeScript.Command) {
					// Make sure we only run script once.
					must.False(t, gotScript)
					gotScript = true
				}
			case <-harness.mockHooks.SignalCh:
				// Make sure we only signal once.
				must.False(t, gotSignal)
				gotSignal = true
			case <-timeout:
				t.Fatal(t, "timeout waiting for script and signal")
			}

			if gotScript && gotSignal {
				break
			}
		}
	})
}

// TestTaskTemplateManager_FiltersEnvVars asserts that we only render
// environment variables found in task env-vars and not read the nomad host
// process environment variables.  nomad host process environment variables are
// to be treated the same as not found environment variables.
func TestTaskTemplateManager_FiltersEnvVars(t *testing.T) {

	t.Setenv("NOMAD_TASK_NAME", "should be overridden by task")

	testenv := "TESTENV_" + strings.ReplaceAll(uuid.Generate(), "-", "")
	t.Setenv(testenv, "MY_TEST_VALUE")

	// Make a template that will render immediately
	content := `Hello Nomad Task: {{env "NOMAD_TASK_NAME"}}
TEST_ENV: {{ env "` + testenv + `" }}
TEST_ENV_NOT_FOUND: {{env "` + testenv + `_NOTFOUND" }}`
	expected := fmt.Sprintf("Hello Nomad Task: %s\nTEST_ENV: \nTEST_ENV_NOT_FOUND: ", TestTaskName)

	file := "my.tmpl"
	template := &structs.Template{
		EmbeddedTmpl: content,
		DestPath:     file,
		ChangeMode:   structs.TemplateChangeModeNoop,
	}

	harness := newTestHarness(t, []*structs.Template{template}, false, false)
	harness.config.TemplateConfig.DisableSandbox = true // no real alloc in this test
	harness.start(t)
	defer harness.stop()

	// Wait for the unblock
	select {
	case <-harness.mockHooks.UnblockCh:
	case <-time.After(time.Duration(5*testutil.TestMultiplier()) * time.Second):
		t.Fatal(t, "Task unblock should have been called")
	}

	// Check the file is there
	path := filepath.Join(harness.taskDir, file)
	raw, err := os.ReadFile(path)
	must.NoError(t, err)

	must.Eq(t, expected, string(raw))
}

// TestTaskTemplateManager_Env asserts templates with the env flag set are read
// into the task's environment.
func TestTaskTemplateManager_Env(t *testing.T) {
	ci.Parallel(t)
	clienttestutil.RequireConsul(t)

	template := &structs.Template{
		EmbeddedTmpl: `
# Comment lines are ok

FOO=bar
foo=123
ANYTHING_goes=Spaces are=ok!
`,
		DestPath:   "test.env",
		ChangeMode: structs.TemplateChangeModeNoop,
		Envvars:    true,
	}
	harness := newTestHarness(t, []*structs.Template{template}, true, false)
	harness.config.TemplateConfig.DisableSandbox = true // no real alloc in this test
	harness.start(t)
	defer harness.stop()

	// Wait a little
	select {
	case <-harness.mockHooks.UnblockCh:
	case <-time.After(time.Duration(2*testutil.TestMultiplier()) * time.Second):
		t.Fatalf("Should have received unblock: %+v", harness.mockHooks)
	}

	// Validate environment
	env := harness.envBuilder.Build().Map()
	if len(env) < 3 {
		t.Fatalf("expected at least 3 env vars but found %d:\n%#v\n", len(env), env)
	}
	if env["FOO"] != "bar" {
		t.Errorf("expected FOO=bar but found %q", env["FOO"])
	}
	if env["foo"] != "123" {
		t.Errorf("expected foo=123 but found %q", env["foo"])
	}
	if env["ANYTHING_goes"] != "Spaces are=ok!" {
		t.Errorf("expected ANYTHING_GOES='Spaces are ok!' but found %q", env["ANYTHING_goes"])
	}
}

// TestTaskTemplateManager_Env_Missing asserts the core env
// template processing function returns errors when files don't exist
func TestTaskTemplateManager_Env_Missing(t *testing.T) {
	ci.Parallel(t)
	d := t.TempDir()

	// Fake writing the file so we don't have to run the whole template manager
	err := os.WriteFile(filepath.Join(d, "exists.env"), []byte("FOO=bar\n"), 0644)
	if err != nil {
		t.Fatalf("error writing template file: %v", err)
	}

	templates := []*structs.Template{
		{
			EmbeddedTmpl: "FOO=bar\n",
			DestPath:     "exists.env",
			Envvars:      true,
		},
		{
			EmbeddedTmpl: "WHAT=ever\n",
			DestPath:     "missing.env",
			Envvars:      true,
		},
	}

	taskEnv := taskenv.NewEmptyBuilder().SetClientTaskRoot(d).Build()
	if vars, err := loadTemplateEnv(templates, taskEnv); err == nil {
		t.Fatalf("expected an error but instead got env vars: %#v", vars)
	}
}

// TestTaskTemplateManager_Env_InterpolatedDest asserts the core env
// template processing function handles interpolated destinations
func TestTaskTemplateManager_Env_InterpolatedDest(t *testing.T) {
	ci.Parallel(t)

	d := t.TempDir()

	// Fake writing the file so we don't have to run the whole template manager
	err := os.WriteFile(filepath.Join(d, "exists.env"), []byte("FOO=bar\n"), 0644)
	if err != nil {
		t.Fatalf("error writing template file: %v", err)
	}

	templates := []*structs.Template{
		{
			EmbeddedTmpl: "FOO=bar\n",
			DestPath:     "${NOMAD_META_path}.env",
			Envvars:      true,
		},
	}

	// Build the env
	taskEnv := taskenv.NewTaskEnv(
		map[string]string{"NOMAD_META_path": "exists"},
		map[string]string{"NOMAD_META_path": "exists"},
		map[string]string{},
		map[string]string{},
		d, "")

	vars, err := loadTemplateEnv(templates, taskEnv)
	must.NoError(t, err)
	must.MapContainsKey(t, vars, "FOO")
	must.Eq(t, "bar", vars["FOO"])
}

// TestTaskTemplateManager_Env_Multi asserts the core env
// template processing function returns combined env vars from multiple
// templates correctly.
func TestTaskTemplateManager_Env_Multi(t *testing.T) {
	ci.Parallel(t)
	d := t.TempDir()

	// Fake writing the files so we don't have to run the whole template manager
	err := os.WriteFile(filepath.Join(d, "zzz.env"), []byte("FOO=bar\nSHARED=nope\n"), 0644)
	if err != nil {
		t.Fatalf("error writing template file 1: %v", err)
	}
	err = os.WriteFile(filepath.Join(d, "aaa.env"), []byte("BAR=foo\nSHARED=yup\n"), 0644)
	if err != nil {
		t.Fatalf("error writing template file 2: %v", err)
	}

	// Templates will get loaded in order (not alpha sorted)
	templates := []*structs.Template{
		{
			DestPath: "zzz.env",
			Envvars:  true,
		},
		{
			DestPath: "aaa.env",
			Envvars:  true,
		},
	}

	taskEnv := taskenv.NewEmptyBuilder().SetClientTaskRoot(d).Build()
	vars, err := loadTemplateEnv(templates, taskEnv)
	if err != nil {
		t.Fatalf("expected no error: %v", err)
	}
	if vars["FOO"] != "bar" {
		t.Errorf("expected FOO=bar but found %q", vars["FOO"])
	}
	if vars["BAR"] != "foo" {
		t.Errorf("expected BAR=foo but found %q", vars["BAR"])
	}
	if vars["SHARED"] != "yup" {
		t.Errorf("expected FOO=bar but found %q", vars["yup"])
	}
}

func TestTaskTemplateManager_Rerender_Env(t *testing.T) {
	ci.Parallel(t)
	clienttestutil.RequireConsul(t)

	// Make a template that renders based on a key in Consul and sends restart
	key1 := "bam"
	key2 := "bar"
	content1_1 := "cat"
	content1_2 := "dog"
	t1 := &structs.Template{
		EmbeddedTmpl: `
FOO={{key "bam"}}
`,
		DestPath:   "test.env",
		ChangeMode: structs.TemplateChangeModeRestart,
		Envvars:    true,
	}
	t2 := &structs.Template{
		EmbeddedTmpl: `
BAR={{key "bar"}}
`,
		DestPath:   "test2.env",
		ChangeMode: structs.TemplateChangeModeRestart,
		Envvars:    true,
	}

	harness := newTestHarness(t, []*structs.Template{t1, t2}, true, false)
	harness.start(t)
	defer harness.stop()

	// Ensure no unblock
	select {
	case <-harness.mockHooks.UnblockCh:
		t.Fatalf("Task unblock should have not have been called")
	case <-time.After(time.Duration(1*testutil.TestMultiplier()) * time.Second):
	}

	// Write the key to Consul
	harness.consul.SetKV(t, key1, []byte(content1_1))
	harness.consul.SetKV(t, key2, []byte(content1_1))

	// Wait for the unblock
	select {
	case <-harness.mockHooks.UnblockCh:
	case <-time.After(time.Duration(5*testutil.TestMultiplier()) * time.Second):
		t.Fatalf("Task unblock should have been called")
	}

	env := harness.envBuilder.Build().Map()
	if v, ok := env["FOO"]; !ok || v != content1_1 {
		t.Fatalf("Bad env for FOO: %v %v", v, ok)
	}
	if v, ok := env["BAR"]; !ok || v != content1_1 {
		t.Fatalf("Bad env for BAR: %v %v", v, ok)
	}

	// Update the keys in Consul
	harness.consul.SetKV(t, key1, []byte(content1_2))

	// Wait for restart
	timeout := time.After(time.Duration(1*testutil.TestMultiplier()) * time.Second)
OUTER:
	for {
		select {
		case <-harness.mockHooks.RestartCh:
			break OUTER
		case <-harness.mockHooks.SignalCh:
			t.Fatalf("Signal with restart policy: %+v", harness.mockHooks)
		case <-timeout:
			t.Fatalf("Should have received a restart: %+v", harness.mockHooks)
		}
	}

	env = harness.envBuilder.Build().Map()
	if v, ok := env["FOO"]; !ok || v != content1_2 {
		t.Fatalf("Bad env for FOO: %v %v", v, ok)
	}
	if v, ok := env["BAR"]; !ok || v != content1_1 {
		t.Fatalf("Bad env for BAR: %v %v", v, ok)
	}
}

// TestTaskTemplateManager_Config_ServerName asserts the tls_server_name
// setting is propagated to consul-template's configuration. See #2776
func TestTaskTemplateManager_Config_ServerName(t *testing.T) {
	ci.Parallel(t)
	c := config.DefaultConfig()
	c.Node = mock.Node()
	c.VaultConfigs = map[string]*sconfig.VaultConfig{
		structs.VaultDefaultCluster: {
			Enabled:       pointer.Of(true),
			Addr:          "https://localhost/",
			TLSServerName: "notlocalhost",
		},
	}

	config := &TaskTemplateManagerConfig{
		ClientConfig: c,
		VaultToken:   "token",
		VaultConfig:  c.GetDefaultVault(),
		TaskID:       uuid.Generate(),
	}
	ctconf, err := newRunnerConfig(config, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if *ctconf.Vault.SSL.ServerName != c.GetDefaultVault().TLSServerName {
		t.Fatalf("expected %q but found %q", c.GetDefaultVault().TLSServerName, *ctconf.Vault.SSL.ServerName)
	}
}

// TestTaskTemplateManager_Config_VaultNamespace asserts the Vault namespace setting is
// propagated to consul-template's configuration.
func TestTaskTemplateManager_Config_VaultNamespace(t *testing.T) {
	ci.Parallel(t)

	testNS := "test-namespace"
	c := config.DefaultConfig()
	c.Node = mock.Node()
	c.VaultConfigs = map[string]*sconfig.VaultConfig{
		structs.VaultDefaultCluster: {
			Enabled:       pointer.Of(true),
			Addr:          "https://localhost/",
			TLSServerName: "notlocalhost",
			Namespace:     testNS,
		},
	}

	alloc := mock.Alloc()
	config := &TaskTemplateManagerConfig{
		ClientConfig: c,
		VaultToken:   "token",
		VaultConfig:  c.GetDefaultVault(),
		EnvBuilder:   taskenv.NewBuilder(c.Node, alloc, alloc.Job.TaskGroups[0].Tasks[0], c.Region),
		TaskID:       uuid.Generate(),
	}

	ctmplMapping, err := parseTemplateConfigs(config)
	must.NoError(t, err, must.Sprint("parsing templates"))

	ctconf, err := newRunnerConfig(config, ctmplMapping)
	must.NoError(t, err, must.Sprint("building runner config"))
	must.Eq(t, testNS, *ctconf.Vault.Namespace, must.Sprintf("Vault Namespace Value"))
}

// TestTaskTemplateManager_Config_VaultNamespace asserts the Vault namespace setting is
// propagated to consul-template's configuration.
func TestTaskTemplateManager_Config_VaultNamespace_TaskOverride(t *testing.T) {
	ci.Parallel(t)

	testNS := "test-namespace"
	c := config.DefaultConfig()
	c.Node = mock.Node()
	c.VaultConfigs = map[string]*sconfig.VaultConfig{
		structs.VaultDefaultCluster: {
			Enabled:       pointer.Of(true),
			Addr:          "https://localhost/",
			TLSServerName: "notlocalhost",
			Namespace:     testNS,
		},
	}

	alloc := mock.Alloc()
	overriddenNS := "new-namespace"

	// Set the template manager config vault namespace
	config := &TaskTemplateManagerConfig{
		ClientConfig:   c,
		VaultToken:     "token",
		VaultConfig:    c.GetDefaultVault(),
		VaultNamespace: overriddenNS,
		EnvBuilder:     taskenv.NewBuilder(c.Node, alloc, alloc.Job.TaskGroups[0].Tasks[0], c.Region),
		TaskID:         uuid.Generate(),
	}

	ctmplMapping, err := parseTemplateConfigs(config)
	must.NoError(t, err, must.Sprint("parsing templates"))

	ctconf, err := newRunnerConfig(config, ctmplMapping)
	must.NoError(t, err, must.Sprint("building runner config"))
	must.Eq(t, overriddenNS, *ctconf.Vault.Namespace, must.Sprintf("Vault Namespace Value"))
}

// TestTaskTemplateManager_Escapes asserts that when sandboxing is enabled
// interpolated paths are not incorrectly treated as escaping the alloc dir.
func TestTaskTemplateManager_Escapes(t *testing.T) {
	ci.Parallel(t)

	// the specific files paths are different on Linux vs Windows
	// TODO(tgross): rewrite this test to allow for platform-specific paths
	clienttestutil.RequireNotWindows(t)

	clientConf := config.DefaultConfig()
	must.False(t, clientConf.TemplateConfig.DisableSandbox, must.Sprint("expected sandbox to be enabled"))

	// Set a fake alloc dir to make test output more realistic
	clientConf.AllocDir = "/fake/allocdir"

	clientConf.Node = mock.Node()
	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	logger := testlog.HCLogger(t)
	allocDir := allocdir.NewAllocDir(logger, clientConf.AllocDir, clientConf.AllocMountsDir, alloc.ID)
	taskDir := allocDir.NewTaskDir(task)

	containerEnv := func() *taskenv.Builder {
		// To emulate a Docker or exec tasks we must copy the
		// Set{Alloc,Task,Secrets}Dir logic in taskrunner/task_dir_hook.go
		b := taskenv.NewBuilder(clientConf.Node, alloc, task, clientConf.Region)
		b.SetAllocDir(allocdir.SharedAllocContainerPath)
		b.SetTaskLocalDir(allocdir.TaskLocalContainerPath)
		b.SetSecretsDir(allocdir.TaskSecretsContainerPath)
		b.SetClientTaskRoot(taskDir.Dir)
		b.SetClientSharedAllocDir(taskDir.SharedAllocDir)
		b.SetClientTaskLocalDir(taskDir.LocalDir)
		b.SetClientTaskSecretsDir(taskDir.SecretsDir)
		return b
	}

	rawExecEnv := func() *taskenv.Builder {
		// To emulate a unisolated tasks we must copy the
		// Set{Alloc,Task,Secrets}Dir logic in taskrunner/task_dir_hook.go
		b := taskenv.NewBuilder(clientConf.Node, alloc, task, clientConf.Region)
		b.SetAllocDir(taskDir.SharedAllocDir)
		b.SetTaskLocalDir(taskDir.LocalDir)
		b.SetSecretsDir(taskDir.SecretsDir)
		b.SetClientTaskRoot(taskDir.Dir)
		b.SetClientSharedAllocDir(taskDir.SharedAllocDir)
		b.SetClientTaskLocalDir(taskDir.LocalDir)
		b.SetClientTaskSecretsDir(taskDir.SecretsDir)
		return b
	}

	cases := []struct {
		Name   string
		Config func() *TaskTemplateManagerConfig

		// Expected paths to be returned if Err is nil
		SourcePath string
		DestPath   string

		// Err is the expected error to be returned or nil
		Err error
	}{
		{
			Name: "ContainerOk",
			Config: func() *TaskTemplateManagerConfig {
				return &TaskTemplateManagerConfig{
					ClientConfig: clientConf,
					TaskDir:      taskDir.Dir,
					EnvBuilder:   containerEnv(),
					Templates: []*structs.Template{
						{
							SourcePath: "${NOMAD_TASK_DIR}/src",
							DestPath:   "${NOMAD_SECRETS_DIR}/dst",
						},
					},
				}
			},
			SourcePath: filepath.Join(taskDir.Dir, "local/src"),
			DestPath:   filepath.Join(taskDir.Dir, "secrets/dst"),
		},
		{
			Name: "ContainerSrcEscapesErr",
			Config: func() *TaskTemplateManagerConfig {
				return &TaskTemplateManagerConfig{
					ClientConfig: clientConf,
					TaskDir:      taskDir.Dir,
					EnvBuilder:   containerEnv(),
					Templates: []*structs.Template{
						{
							SourcePath: "/etc/src_escapes",
							DestPath:   "${NOMAD_SECRETS_DIR}/dst",
						},
					},
				}
			},
			Err: sourceEscapesErr,
		},
		{
			Name: "ContainerSrcEscapesOk",
			Config: func() *TaskTemplateManagerConfig {
				unsafeConf := clientConf.Copy()
				unsafeConf.TemplateConfig.DisableSandbox = true
				return &TaskTemplateManagerConfig{
					ClientConfig: unsafeConf,
					TaskDir:      taskDir.Dir,
					EnvBuilder:   containerEnv(),
					Templates: []*structs.Template{
						{
							SourcePath: "/etc/src_escapes_ok",
							DestPath:   "${NOMAD_SECRETS_DIR}/dst",
						},
					},
				}
			},
			SourcePath: "/etc/src_escapes_ok",
			DestPath:   filepath.Join(taskDir.Dir, "secrets/dst"),
		},
		{
			Name: "ContainerDstAbsoluteOk",
			Config: func() *TaskTemplateManagerConfig {
				return &TaskTemplateManagerConfig{
					ClientConfig: clientConf,
					TaskDir:      taskDir.Dir,
					EnvBuilder:   containerEnv(),
					Templates: []*structs.Template{
						{
							SourcePath: "${NOMAD_TASK_DIR}/src",
							DestPath:   "/etc/absolutely_relative",
						},
					},
				}
			},
			SourcePath: filepath.Join(taskDir.Dir, "local/src"),
			DestPath:   filepath.Join(taskDir.Dir, "etc/absolutely_relative"),
		},
		{
			Name: "ContainerDstAbsoluteEscapesErr",
			Config: func() *TaskTemplateManagerConfig {
				return &TaskTemplateManagerConfig{
					ClientConfig: clientConf,
					TaskDir:      taskDir.Dir,
					EnvBuilder:   containerEnv(),
					Templates: []*structs.Template{
						{
							SourcePath: "${NOMAD_TASK_DIR}/src",
							DestPath:   "../escapes",
						},
					},
				}
			},
			Err: destEscapesErr,
		},
		{
			Name: "ContainerDstAbsoluteEscapesOk",
			Config: func() *TaskTemplateManagerConfig {
				unsafeConf := clientConf.Copy()
				unsafeConf.TemplateConfig.DisableSandbox = true
				return &TaskTemplateManagerConfig{
					ClientConfig: unsafeConf,
					TaskDir:      taskDir.Dir,
					EnvBuilder:   containerEnv(),
					Templates: []*structs.Template{
						{
							SourcePath: "${NOMAD_TASK_DIR}/src",
							DestPath:   "../escapes",
						},
					},
				}
			},
			SourcePath: filepath.Join(taskDir.Dir, "local/src"),
			DestPath:   filepath.Join(taskDir.Dir, "..", "escapes"),
		},
		//TODO: Fix this test. I *think* it should pass. The double
		//      joining of the task dir onto the destination seems like
		//      a bug. https://github.com/hashicorp/nomad/issues/9389
		{
			Name: "RawExecOk",
			Config: func() *TaskTemplateManagerConfig {
				return &TaskTemplateManagerConfig{
					ClientConfig: clientConf,
					TaskDir:      taskDir.Dir,
					EnvBuilder:   rawExecEnv(),
					Templates: []*structs.Template{
						{
							SourcePath: "${NOMAD_TASK_DIR}/src",
							DestPath:   "${NOMAD_SECRETS_DIR}/dst",
						},
					},
				}
			},
			SourcePath: filepath.Join(taskDir.Dir, "local/src"),
			DestPath:   filepath.Join(taskDir.Dir, "secrets/dst"),
		},
		{
			Name: "RawExecSrcEscapesErr",
			Config: func() *TaskTemplateManagerConfig {
				return &TaskTemplateManagerConfig{
					ClientConfig: clientConf,
					TaskDir:      taskDir.Dir,
					EnvBuilder:   rawExecEnv(),
					Templates: []*structs.Template{
						{
							SourcePath: "/etc/src_escapes",
							DestPath:   "${NOMAD_SECRETS_DIR}/dst",
						},
					},
				}
			},
			Err: sourceEscapesErr,
		},
		{
			Name: "RawExecDstAbsoluteOk",
			Config: func() *TaskTemplateManagerConfig {
				return &TaskTemplateManagerConfig{
					ClientConfig: clientConf,
					TaskDir:      taskDir.Dir,
					EnvBuilder:   rawExecEnv(),
					Templates: []*structs.Template{
						{
							SourcePath: "${NOMAD_TASK_DIR}/src",
							DestPath:   "/etc/absolutely_relative",
						},
					},
				}
			},
			SourcePath: filepath.Join(taskDir.Dir, "local/src"),
			DestPath:   filepath.Join(taskDir.Dir, "etc/absolutely_relative"),
		},
	}

	for i := range cases {
		tc := cases[i]
		t.Run(tc.Name, func(t *testing.T) {
			config := tc.Config()
			config.TaskID = uuid.Generate()
			mapping, err := parseTemplateConfigs(config)
			if tc.Err == nil {
				// Ok path
				must.NoError(t, err)
				must.NotNil(t, mapping)
				must.MapLen(t, 1, mapping)
				for k := range mapping {
					must.Eq(t, tc.SourcePath, *k.Source)
					must.Eq(t, tc.DestPath, *k.Destination)
					t.Logf("Rendering %s => %s", *k.Source, *k.Destination)
				}
			} else {
				// Err path
				test.EqError(t, err, tc.Err.Error())
				must.Nil(t, mapping)
			}

		})
	}
}

func TestTaskTemplateManager_BlockedEvents(t *testing.T) {
	// The tests sets a template that need keys 0, 1, 2, 3, 4,
	// then subsequently sets 0, 1, 2 keys
	// then asserts that templates are still blocked on 3 and 4,
	// and check that we got the relevant task events
	ci.Parallel(t)
	clienttestutil.RequireConsul(t)

	// Make a template that will render based on a key in Consul
	var embedded string
	for i := 0; i < 5; i++ {
		embedded += fmt.Sprintf(`{{key "%d"}}`, i)
	}

	file := "my.tmpl"
	template := &structs.Template{
		EmbeddedTmpl: embedded,
		DestPath:     file,
		ChangeMode:   structs.TemplateChangeModeNoop,
	}

	harness := newTestHarness(t, []*structs.Template{template}, true, false)
	harness.setEmitRate(100 * time.Millisecond)
	harness.start(t)
	defer harness.stop()

	missingKeys := func(e *structs.TaskEvent) ([]string, int) {
		missingRexp := regexp.MustCompile(`kv.block\(([0-9]*)\)`)
		moreRexp := regexp.MustCompile(`and ([0-9]*) more`)

		missingMatch := missingRexp.FindAllStringSubmatch(e.DisplayMessage, -1)
		moreMatch := moreRexp.FindAllStringSubmatch(e.DisplayMessage, -1)

		missing := make([]string, len(missingMatch))
		for i, v := range missingMatch {
			missing[i] = v[1]
		}
		sort.Strings(missing)

		more := 0
		if len(moreMatch) != 0 {
			more, _ = strconv.Atoi(moreMatch[0][1])
		}
		return missing, more

	}

	// Ensure that we get a blocked event
	select {
	case <-harness.mockHooks.UnblockCh:
		t.Fatalf("Task unblock should have not have been called")
	case <-harness.mockHooks.EmitEventCh:
	case <-time.After(time.Duration(1*testutil.TestMultiplier()) * time.Second):
		t.Fatalf("timeout")
	}

	// Check to see we got a correct message
	// assert that all 0-4 keys are missing
	must.Len(t, 1, harness.mockHooks.Events())
	t.Logf("first message: %v", harness.mockHooks.Events()[0])
	missing, more := missingKeys(harness.mockHooks.Events()[0])
	must.Eq(t, 5, len(missing)+more)
	must.StrContains(t, harness.mockHooks.Events()[0].DisplayMessage, "and 2 more")

	// Write 0-2 keys to Consul
	for i := 0; i < 3; i++ {
		harness.consul.SetKV(t, fmt.Sprintf("%d", i), []byte{0xa})
	}

	// Ensure that we get a blocked event
	isExpectedFinalEvent := func(e *structs.TaskEvent) bool {
		missing, more := missingKeys(e)
		return reflect.DeepEqual(missing, []string{"3", "4"}) && more == 0
	}
	timeout := time.After(time.Second * time.Duration(testutil.TestMultiplier()))
WAIT_LOOP:
	for {
		select {
		case <-harness.mockHooks.UnblockCh:
			t.Errorf("Task unblock should have not have been called")
		case e := <-harness.mockHooks.EmitEventCh:
			t.Logf("received event: %v", e.DisplayMessage)
			if isExpectedFinalEvent(e) {
				break WAIT_LOOP
			}
		case <-timeout:
			t.Errorf("timeout")
		}
	}

	// Check to see we got a correct message
	event := harness.mockHooks.Events()[len(harness.mockHooks.Events())-1]
	if !isExpectedFinalEvent(event) {
		t.Logf("received all events: %v", pretty.Sprint(harness.mockHooks.Events))

		t.Fatalf("bad event, expected only 3 and 5 blocked got: %q", event.DisplayMessage)
	}
}

// TestTaskTemplateManager_ClientTemplateConfig_Set asserts that all client level
// configuration is accurately mapped from the client to the TaskTemplateManager
// and that any operator defined boundaries are enforced.
func TestTaskTemplateManager_ClientTemplateConfig_Set(t *testing.T) {
	ci.Parallel(t)

	testNS := "test-namespace"

	clientConfig := config.DefaultConfig()
	clientConfig.Node = mock.Node()

	clientConfig.VaultConfigs = map[string]*sconfig.VaultConfig{
		structs.VaultDefaultCluster: {
			Enabled:   pointer.Of(true),
			Namespace: testNS,
		},
	}
	clientConfig.ConsulConfigs = map[string]*sconfig.ConsulConfig{
		structs.ConsulDefaultCluster: {
			Namespace: testNS,
		}}

	// helper to reduce boilerplate
	waitConfig := &config.WaitConfig{
		Min: pointer.Of(5 * time.Second),
		Max: pointer.Of(10 * time.Second),
	}
	// helper to reduce boilerplate
	retryConfig := &config.RetryConfig{
		Attempts:   pointer.Of(5),
		Backoff:    pointer.Of(5 * time.Second),
		MaxBackoff: pointer.Of(20 * time.Second),
	}

	clientConfig.TemplateConfig.MaxStale = pointer.Of(5 * time.Second)
	clientConfig.TemplateConfig.BlockQueryWaitTime = pointer.Of(60 * time.Second)
	clientConfig.TemplateConfig.Wait = waitConfig.Copy()
	clientConfig.TemplateConfig.ConsulRetry = retryConfig.Copy()
	clientConfig.TemplateConfig.VaultRetry = retryConfig.Copy()
	clientConfig.TemplateConfig.NomadRetry = retryConfig.Copy()

	alloc := mock.Alloc()
	allocWithOverride := mock.Alloc()
	allocWithOverride.Job.TaskGroups[0].Tasks[0].Templates = []*structs.Template{
		{
			Wait: &structs.WaitConfig{
				Min: pointer.Of(2 * time.Second),
				Max: pointer.Of(12 * time.Second),
			},
		},
	}

	cases := []struct {
		Name                   string
		ClientTemplateConfig   *config.ClientTemplateConfig
		TTMConfig              *TaskTemplateManagerConfig
		ExpectedRunnerConfig   *config.Config
		ExpectedTemplateConfig *templateconfig.TemplateConfig
	}{
		{
			"basic-wait-config",
			&config.ClientTemplateConfig{
				MaxStale:           pointer.Of(5 * time.Second),
				BlockQueryWaitTime: pointer.Of(60 * time.Second),
				Wait:               waitConfig.Copy(),
				ConsulRetry:        retryConfig.Copy(),
				VaultRetry:         retryConfig.Copy(),
				NomadRetry:         retryConfig.Copy(),
			},
			&TaskTemplateManagerConfig{
				ClientConfig: clientConfig,
				ConsulConfig: clientConfig.GetDefaultConsul(),
				VaultToken:   "token",
				VaultConfig:  clientConfig.GetDefaultVault(),
				EnvBuilder:   taskenv.NewBuilder(clientConfig.Node, alloc, alloc.Job.TaskGroups[0].Tasks[0], clientConfig.Region),
			},
			&config.Config{
				TemplateConfig: &config.ClientTemplateConfig{
					MaxStale:           pointer.Of(5 * time.Second),
					BlockQueryWaitTime: pointer.Of(60 * time.Second),
					Wait:               waitConfig.Copy(),
					ConsulRetry:        retryConfig.Copy(),
					VaultRetry:         retryConfig.Copy(),
					NomadRetry:         retryConfig.Copy(),
				},
			},
			&templateconfig.TemplateConfig{
				Wait: &templateconfig.WaitConfig{
					Enabled: pointer.Of(true),
					Min:     pointer.Of(5 * time.Second),
					Max:     pointer.Of(10 * time.Second),
				},
			},
		},
		{
			"template-override",
			&config.ClientTemplateConfig{
				MaxStale:           pointer.Of(5 * time.Second),
				BlockQueryWaitTime: pointer.Of(60 * time.Second),
				Wait:               waitConfig.Copy(),
				ConsulRetry:        retryConfig.Copy(),
				VaultRetry:         retryConfig.Copy(),
				NomadRetry:         retryConfig.Copy(),
			},
			&TaskTemplateManagerConfig{
				ClientConfig: clientConfig,
				ConsulConfig: clientConfig.GetDefaultConsul(),
				VaultToken:   "token",
				VaultConfig:  clientConfig.GetDefaultVault(),
				EnvBuilder:   taskenv.NewBuilder(clientConfig.Node, allocWithOverride, allocWithOverride.Job.TaskGroups[0].Tasks[0], clientConfig.Region),
			},
			&config.Config{
				TemplateConfig: &config.ClientTemplateConfig{
					MaxStale:           pointer.Of(5 * time.Second),
					BlockQueryWaitTime: pointer.Of(60 * time.Second),
					Wait:               waitConfig.Copy(),
					ConsulRetry:        retryConfig.Copy(),
					VaultRetry:         retryConfig.Copy(),
					NomadRetry:         retryConfig.Copy(),
				},
			},
			&templateconfig.TemplateConfig{
				Wait: &templateconfig.WaitConfig{
					Enabled: pointer.Of(true),
					Min:     pointer.Of(2 * time.Second),
					Max:     pointer.Of(12 * time.Second),
				},
			},
		},
		{
			"bounds-override",
			&config.ClientTemplateConfig{
				MaxStale:           pointer.Of(5 * time.Second),
				BlockQueryWaitTime: pointer.Of(60 * time.Second),
				Wait:               waitConfig.Copy(),
				WaitBounds: &config.WaitConfig{
					Min: pointer.Of(3 * time.Second),
					Max: pointer.Of(11 * time.Second),
				},
				ConsulRetry: retryConfig.Copy(),
				VaultRetry:  retryConfig.Copy(),
				NomadRetry:  retryConfig.Copy(),
			},
			&TaskTemplateManagerConfig{
				ClientConfig: clientConfig,
				ConsulConfig: clientConfig.GetDefaultConsul(),
				VaultToken:   "token",
				VaultConfig:  clientConfig.GetDefaultVault(),
				EnvBuilder:   taskenv.NewBuilder(clientConfig.Node, allocWithOverride, allocWithOverride.Job.TaskGroups[0].Tasks[0], clientConfig.Region),
				Templates: []*structs.Template{
					{
						Wait: &structs.WaitConfig{
							Min: pointer.Of(2 * time.Second),
							Max: pointer.Of(12 * time.Second),
						},
					},
				},
			},
			&config.Config{
				TemplateConfig: &config.ClientTemplateConfig{
					MaxStale:           pointer.Of(5 * time.Second),
					BlockQueryWaitTime: pointer.Of(60 * time.Second),
					Wait:               waitConfig.Copy(),
					WaitBounds: &config.WaitConfig{
						Min: pointer.Of(3 * time.Second),
						Max: pointer.Of(11 * time.Second),
					},
					ConsulRetry: retryConfig.Copy(),
					VaultRetry:  retryConfig.Copy(),
					NomadRetry:  retryConfig.Copy(),
				},
			},
			&templateconfig.TemplateConfig{
				Wait: &templateconfig.WaitConfig{
					Enabled: pointer.Of(true),
					Min:     pointer.Of(3 * time.Second),
					Max:     pointer.Of(11 * time.Second),
				},
			},
		},
	}

	for _, _case := range cases {
		t.Run(_case.Name, func(t *testing.T) {
			// monkey patch the client config with the version of the ClientTemplateConfig we want to test.
			_case.TTMConfig.ClientConfig.TemplateConfig = _case.ClientTemplateConfig
			_case.TTMConfig.TaskID = uuid.Generate()
			templateMapping, err := parseTemplateConfigs(_case.TTMConfig)
			must.NoError(t, err)

			runnerConfig, err := newRunnerConfig(_case.TTMConfig, templateMapping)
			must.NoError(t, err)

			// Direct properties
			must.Eq(t, *_case.ExpectedRunnerConfig.TemplateConfig.MaxStale, *runnerConfig.MaxStale)
			must.Eq(t, *_case.ExpectedRunnerConfig.TemplateConfig.BlockQueryWaitTime, *runnerConfig.BlockQueryWaitTime)
			// WaitConfig
			must.Eq(t, *_case.ExpectedRunnerConfig.TemplateConfig.Wait.Min, *runnerConfig.Wait.Min)
			must.Eq(t, *_case.ExpectedRunnerConfig.TemplateConfig.Wait.Max, *runnerConfig.Wait.Max)
			// Consul Retry
			must.NotNil(t, runnerConfig.Consul)
			must.NotNil(t, runnerConfig.Consul.Retry)
			must.Eq(t, *_case.ExpectedRunnerConfig.TemplateConfig.ConsulRetry.Attempts, *runnerConfig.Consul.Retry.Attempts)
			must.Eq(t, *_case.ExpectedRunnerConfig.TemplateConfig.ConsulRetry.Backoff, *runnerConfig.Consul.Retry.Backoff)
			must.Eq(t, *_case.ExpectedRunnerConfig.TemplateConfig.ConsulRetry.MaxBackoff, *runnerConfig.Consul.Retry.MaxBackoff)
			// Vault Retry
			must.NotNil(t, runnerConfig.Vault)
			must.NotNil(t, runnerConfig.Vault.Retry)
			must.Eq(t, *_case.ExpectedRunnerConfig.TemplateConfig.VaultRetry.Attempts, *runnerConfig.Vault.Retry.Attempts)
			must.Eq(t, *_case.ExpectedRunnerConfig.TemplateConfig.VaultRetry.Backoff, *runnerConfig.Vault.Retry.Backoff)
			must.Eq(t, *_case.ExpectedRunnerConfig.TemplateConfig.VaultRetry.MaxBackoff, *runnerConfig.Vault.Retry.MaxBackoff)
			// Nomad Retry
			must.NotNil(t, runnerConfig.Nomad)
			must.NotNil(t, runnerConfig.Nomad.Retry)
			must.Eq(t, *_case.ExpectedRunnerConfig.TemplateConfig.NomadRetry.Attempts, *runnerConfig.Nomad.Retry.Attempts)
			must.Eq(t, *_case.ExpectedRunnerConfig.TemplateConfig.NomadRetry.Backoff, *runnerConfig.Nomad.Retry.Backoff)
			must.Eq(t, *_case.ExpectedRunnerConfig.TemplateConfig.NomadRetry.MaxBackoff, *runnerConfig.Nomad.Retry.MaxBackoff)

			// Test that wait_bounds are enforced
			for _, tmpl := range *runnerConfig.Templates {
				must.Eq(t, *_case.ExpectedTemplateConfig.Wait.Enabled, *tmpl.Wait.Enabled)
				must.Eq(t, *_case.ExpectedTemplateConfig.Wait.Min, *tmpl.Wait.Min)
				must.Eq(t, *_case.ExpectedTemplateConfig.Wait.Max, *tmpl.Wait.Max)
			}
		})
	}
}

// TestTaskTemplateManager_Template_Wait_Set asserts that all template level
// configuration is accurately mapped from the template to the TaskTemplateManager's
// template config.
func TestTaskTemplateManager_Template_Wait_Set(t *testing.T) {
	ci.Parallel(t)

	c := config.DefaultConfig()
	c.Node = mock.Node()

	alloc := mock.Alloc()

	ttmConfig := &TaskTemplateManagerConfig{
		ClientConfig: c,
		VaultToken:   "token",
		EnvBuilder:   taskenv.NewBuilder(c.Node, alloc, alloc.Job.TaskGroups[0].Tasks[0], c.Region),
		Templates: []*structs.Template{
			{
				Wait: &structs.WaitConfig{
					Min: pointer.Of(5 * time.Second),
					Max: pointer.Of(10 * time.Second),
				},
			},
		},
		TaskID: uuid.Generate(),
	}

	templateMapping, err := parseTemplateConfigs(ttmConfig)
	must.NoError(t, err)

	for k, _ := range templateMapping {
		must.True(t, *k.Wait.Enabled)
		must.Eq(t, 5*time.Second, *k.Wait.Min)
		must.Eq(t, 10*time.Second, *k.Wait.Max)
	}
}

// TestTaskTemplateManager_Template_ErrMissingKey_Set asserts that all template level
// configuration is accurately mapped from the template to the TaskTemplateManager's
// template config.
func TestTaskTemplateManager_Template_ErrMissingKey_Set(t *testing.T) {
	ci.Parallel(t)

	c := config.DefaultConfig()
	c.Node = mock.Node()

	alloc := mock.Alloc()

	ttmConfig := &TaskTemplateManagerConfig{
		ClientConfig: c,
		VaultToken:   "token",
		EnvBuilder:   taskenv.NewBuilder(c.Node, alloc, alloc.Job.TaskGroups[0].Tasks[0], c.Region),
		Templates: []*structs.Template{
			{
				EmbeddedTmpl:  "test-false",
				ErrMissingKey: false,
			},
			{
				EmbeddedTmpl:  "test-true",
				ErrMissingKey: true,
			},
		},
		TaskID: uuid.Generate(),
	}

	templateMapping, err := parseTemplateConfigs(ttmConfig)
	must.NoError(t, err)

	for k, tmpl := range templateMapping {
		if tmpl.EmbeddedTmpl == "test-false" {
			must.False(t, *k.ErrMissingKey)
		}
		if tmpl.EmbeddedTmpl == "test-true" {
			must.True(t, *k.ErrMissingKey)
		}
	}
}

// TestTaskTemplateManager_writeToFile_Disabled asserts the consul-template function
// writeToFile is disabled by default.
func TestTaskTemplateManager_writeToFile_Disabled(t *testing.T) {
	ci.Parallel(t)

	file := "my.tmpl"
	template := &structs.Template{
		EmbeddedTmpl: `Testing writeToFile...
{{ "if i exist writeToFile is enabled" | writeToFile "/tmp/NOMAD-TEST-SHOULD-NOT-EXIST" "" "" "0644" }}
...done
`,
		DestPath:   file,
		ChangeMode: structs.TemplateChangeModeNoop,
	}

	harness := newTestHarness(t, []*structs.Template{template}, false, false)
	must.NoError(t, harness.startWithErr(), must.Sprint("couldn't setup initial harness"))
	defer harness.stop()

	// Using writeToFile should cause a kill
	select {
	case <-harness.mockHooks.UnblockCh:
		t.Fatalf("Task unblock should have not have been called")
	case <-harness.mockHooks.EmitEventCh:
		t.Fatalf("Task event should not have been emitted")
	case e := <-harness.mockHooks.KillCh:
		must.StrContains(t, e.DisplayMessage, "writeToFile: function is disabled")
	case <-time.After(time.Duration(5*testutil.TestMultiplier()) * time.Second):
		t.Fatalf("timeout")
	}

	// Check the file is not there
	path := filepath.Join(harness.taskDir, file)
	_, err := os.ReadFile(path)
	must.Error(t, err)
}

// TestTaskTemplateManager_writeToFile asserts the consul-template function
// writeToFile can be enabled.
func TestTaskTemplateManager_writeToFile(t *testing.T) {
	ci.Parallel(t)
	clienttestutil.RequireLinux(t)

	cu, err := users.Current()
	must.NoError(t, err)

	file := "my.tmpl"
	template := &structs.Template{
		// EmbeddedTmpl set below as it needs the taskDir
		DestPath:   file,
		ChangeMode: structs.TemplateChangeModeNoop,
	}

	harness := newTestHarness(t, []*structs.Template{template}, false, false)

	// Add template now that we know the taskDir
	harness.templates[0].EmbeddedTmpl = fmt.Sprintf(`Testing writeToFile...
{{ "hello" | writeToFile "%s" "`+cu.Username+`" "`+cu.Gid+`" "0644" }}
...done
`, filepath.Join(harness.taskDir, "writetofile.out"))

	// Enable all funcs
	harness.config.TemplateConfig.FunctionDenylist = []string{}

	must.NoError(t, harness.startWithErr(), must.Sprint("couldn't setup initial harness"))
	defer harness.stop()

	// Using writeToFile should not cause a kill
	select {
	case <-harness.mockHooks.UnblockCh:
	case <-harness.mockHooks.EmitEventCh:
		t.Fatalf("Task event should not have been emitted")
	case e := <-harness.mockHooks.KillCh:
		t.Fatalf("Task should not have been killed: %v", e.DisplayMessage)
	case <-time.After(time.Duration(5*testutil.TestMultiplier()) * time.Second):
		t.Fatalf("timeout")
	}

	// Check the templated file is there
	path := filepath.Join(harness.taskDir, file)
	r, err := os.ReadFile(path)
	must.NoError(t, err)
	must.True(t, bytes.HasSuffix(r, []byte("...done\n")), must.Sprint(string(r)))

	// Check that writeToFile was allowed
	path = filepath.Join(harness.taskDir, "writetofile.out")
	r, err = os.ReadFile(path)
	must.NoError(t, err)
	must.Eq(t, "hello", string(r))
}
