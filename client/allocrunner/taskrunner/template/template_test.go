package template

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	ctestutil "github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	sconfig "github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/hashicorp/nomad/testutil"
	"github.com/kr/pretty"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// TestTaskName is the name of the injected task. It should appear in the
	// environment variable $NOMAD_TASK_NAME
	TestTaskName = "test-task"
)

// MockTaskHooks is a mock of the TaskHooks interface useful for testing
type MockTaskHooks struct {
	Restarts  int
	RestartCh chan struct{}

	Signals    []string
	SignalCh   chan struct{}
	signalLock sync.Mutex

	// SignalError is returned when Signal is called on the mock hook
	SignalError error

	UnblockCh chan struct{}

	KillEvent *structs.TaskEvent
	KillCh    chan struct{}

	Events      []*structs.TaskEvent
	EmitEventCh chan *structs.TaskEvent

	// hasHandle can be set to simulate restoring a task after client restart
	hasHandle bool
}

func NewMockTaskHooks() *MockTaskHooks {
	return &MockTaskHooks{
		UnblockCh:   make(chan struct{}, 1),
		RestartCh:   make(chan struct{}, 1),
		SignalCh:    make(chan struct{}, 1),
		KillCh:      make(chan struct{}, 1),
		EmitEventCh: make(chan *structs.TaskEvent, 1),
	}
}
func (m *MockTaskHooks) Restart(ctx context.Context, event *structs.TaskEvent, failure bool) error {
	m.Restarts++
	select {
	case m.RestartCh <- struct{}{}:
	default:
	}
	return nil
}

func (m *MockTaskHooks) Signal(event *structs.TaskEvent, s string) error {
	m.signalLock.Lock()
	m.Signals = append(m.Signals, s)
	m.signalLock.Unlock()
	select {
	case m.SignalCh <- struct{}{}:
	default:
	}

	return m.SignalError
}

func (m *MockTaskHooks) Kill(ctx context.Context, event *structs.TaskEvent) error {
	m.KillEvent = event
	select {
	case m.KillCh <- struct{}{}:
	default:
	}
	return nil
}

func (m *MockTaskHooks) HasHandle() bool {
	return m.hasHandle
}

func (m *MockTaskHooks) EmitEvent(event *structs.TaskEvent) {
	m.Events = append(m.Events, event)
	select {
	case m.EmitEventCh <- event:
	case <-m.EmitEventCh:
		m.EmitEventCh <- event
	}
}

func (m *MockTaskHooks) SetState(state string, event *structs.TaskEvent) {}

// testHarness is used to test the TaskTemplateManager by spinning up
// Consul/Vault as needed
type testHarness struct {
	manager    *TaskTemplateManager
	mockHooks  *MockTaskHooks
	templates  []*structs.Template
	envBuilder *taskenv.Builder
	node       *structs.Node
	config     *config.Config
	vaultToken string
	taskDir    string
	vault      *testutil.TestVault
	consul     *ctestutil.TestServer
	emitRate   time.Duration
}

// newTestHarness returns a harness starting a dev consul and vault server,
// building the appropriate config and creating a TaskTemplateManager
func newTestHarness(t *testing.T, templates []*structs.Template, consul, vault bool) *testHarness {
	region := "global"
	harness := &testHarness{
		mockHooks: NewMockTaskHooks(),
		templates: templates,
		node:      mock.Node(),
		config: &config.Config{
			Region: region,
			TemplateConfig: &config.ClientTemplateConfig{
				FunctionDenylist: []string{"plugin"},
				DisableSandbox:   false,
			}},
		emitRate: DefaultMaxTemplateEventRate,
	}

	// Build the task environment
	a := mock.Alloc()
	task := a.Job.TaskGroups[0].Tasks[0]
	task.Name = TestTaskName
	harness.envBuilder = taskenv.NewBuilder(harness.node, a, task, region)

	// Make a tempdir
	d, err := ioutil.TempDir("", "ct_test")
	if err != nil {
		t.Fatalf("Failed to make tmpdir: %v", err)
	}
	harness.taskDir = d

	if consul {
		harness.consul, err = ctestutil.NewTestServerConfigT(t, func(c *ctestutil.TestServerConfig) {
			// defaults
		})
		if err != nil {
			t.Fatalf("error starting test Consul server: %v", err)
		}
		harness.config.ConsulConfig = &sconfig.ConsulConfig{
			Addr: harness.consul.HTTPAddr,
		}
	}

	if vault {
		harness.vault = testutil.NewTestVault(t)
		harness.config.VaultConfig = harness.vault.Config
		harness.vaultToken = harness.vault.RootToken
	}

	return harness
}

func (h *testHarness) start(t *testing.T) {
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
		VaultToken:           h.vaultToken,
		TaskDir:              h.taskDir,
		EnvBuilder:           h.envBuilder,
		MaxTemplateEventRate: h.emitRate,
		retryRate:            10 * time.Millisecond,
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
	t.Parallel()
	hooks := NewMockTaskHooks()
	clientConfig := &config.Config{Region: "global"}
	taskDir := "foo"
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
	t.Parallel()
	// Make a template that will render immediately and write it to a tmp file
	f, err := ioutil.TempFile("", "")
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
	raw, err := ioutil.ReadFile(path)
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

	// Test with desination too
	template.SourcePath = f.Name()
	template.DestPath = "../../../../../../" + file
	harness = newTestHarness(t, []*structs.Template{template}, false, false)
	harness.envBuilder = taskenv.NewBuilder(harness.node, a, task, "global")
	err = harness.startWithErr()
	if err == nil || !strings.Contains(err.Error(), "escapes alloc directory") {
		t.Fatalf("Expected directory traversal out of %q via interpolation disallowed for %q: %v",
			harness.taskDir, template.SourcePath, err)
	}

}

func TestTaskTemplateManager_Unblock_Static(t *testing.T) {
	t.Parallel()
	// Make a template that will render immediately
	content := "hello, world!"
	file := "my.tmpl"
	template := &structs.Template{
		EmbeddedTmpl: content,
		DestPath:     file,
		ChangeMode:   structs.TemplateChangeModeNoop,
	}

	harness := newTestHarness(t, []*structs.Template{template}, false, false)
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
	raw, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read rendered template from %q: %v", path, err)
	}

	if s := string(raw); s != content {
		t.Fatalf("Unexpected template data; got %q, want %q", s, content)
	}
}

func TestTaskTemplateManager_Permissions(t *testing.T) {
	t.Parallel()
	// Make a template that will render immediately
	content := "hello, world!"
	file := "my.tmpl"
	template := &structs.Template{
		EmbeddedTmpl: content,
		DestPath:     file,
		ChangeMode:   structs.TemplateChangeModeNoop,
		Perms:        "777",
	}

	harness := newTestHarness(t, []*structs.Template{template}, false, false)
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
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}

	if m := fi.Mode(); m != os.ModePerm {
		t.Fatalf("Got mode %v; want %v", m, os.ModePerm)
	}
}

func TestTaskTemplateManager_Unblock_Static_NomadEnv(t *testing.T) {
	t.Parallel()
	// Make a template that will render immediately
	content := `Hello Nomad Task: {{env "NOMAD_TASK_NAME"}}`
	expected := fmt.Sprintf("Hello Nomad Task: %s", TestTaskName)
	file := "my.tmpl"
	template := &structs.Template{
		EmbeddedTmpl: content,
		DestPath:     file,
		ChangeMode:   structs.TemplateChangeModeNoop,
	}

	harness := newTestHarness(t, []*structs.Template{template}, false, false)
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
	raw, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read rendered template from %q: %v", path, err)
	}

	if s := string(raw); s != expected {
		t.Fatalf("Unexpected template data; got %q, want %q", s, expected)
	}
}

func TestTaskTemplateManager_Unblock_Static_AlreadyRendered(t *testing.T) {
	t.Parallel()
	// Make a template that will render immediately
	content := "hello, world!"
	file := "my.tmpl"
	template := &structs.Template{
		EmbeddedTmpl: content,
		DestPath:     file,
		ChangeMode:   structs.TemplateChangeModeNoop,
	}

	harness := newTestHarness(t, []*structs.Template{template}, false, false)

	// Write the contents
	path := filepath.Join(harness.taskDir, file)
	if err := ioutil.WriteFile(path, []byte(content), 0777); err != nil {
		t.Fatalf("Failed to write data: %v", err)
	}

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
	raw, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read rendered template from %q: %v", path, err)
	}

	if s := string(raw); s != content {
		t.Fatalf("Unexpected template data; got %q, want %q", s, content)
	}
}

func TestTaskTemplateManager_Unblock_Consul(t *testing.T) {
	t.Parallel()
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
	raw, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read rendered template from %q: %v", path, err)
	}

	if s := string(raw); s != content {
		t.Fatalf("Unexpected template data; got %q, want %q", s, content)
	}
}

func TestTaskTemplateManager_Unblock_Vault(t *testing.T) {
	t.Parallel()
	require := require.New(t)
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
	require.NoError(err)

	// Wait for the unblock
	select {
	case <-harness.mockHooks.UnblockCh:
	case <-time.After(time.Duration(5*testutil.TestMultiplier()) * time.Second):
		t.Fatalf("Task unblock should have been called")
	}

	// Check the file is there
	path := filepath.Join(harness.taskDir, file)
	raw, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read rendered template from %q: %v", path, err)
	}

	if s := string(raw); s != content {
		t.Fatalf("Unexpected template data; got %q, want %q", s, content)
	}
}

func TestTaskTemplateManager_Unblock_Multi_Template(t *testing.T) {
	t.Parallel()
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
	raw, err := ioutil.ReadFile(path)
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
	raw, err = ioutil.ReadFile(path)
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
	t.Parallel()
	require := require.New(t)
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
		t.Fatalf("Task unblock should not have been called")
	case <-time.After(time.Duration(1*testutil.TestMultiplier()) * time.Second):
	}

	// Write the secret to Vault
	logical := harness.vault.Client.Logical()
	_, err := logical.Write(vaultPath, map[string]interface{}{"data": map[string]interface{}{key: content}})
	require.NoError(err)

	// Wait for the unblock
	select {
	case <-harness.mockHooks.UnblockCh:
	case <-time.After(time.Duration(5*testutil.TestMultiplier()) * time.Second):
		t.Fatalf("Task unblock should have been called")
	}

	// Check the file is there
	path := filepath.Join(harness.taskDir, file)
	raw, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read rendered template from %q: %v", path, err)
	}

	if s := string(raw); s != content {
		t.Fatalf("Unexpected template data; got %q, want %q", s, content)
	}

	// task is now running
	harness.mockHooks.hasHandle = true

	// simulate a client restart
	harness.manager.Stop()
	harness.mockHooks.UnblockCh = make(chan struct{}, 1)
	harness.start(t)

	// Wait for the unblock
	select {
	case <-harness.mockHooks.UnblockCh:
	case <-time.After(time.Duration(5*testutil.TestMultiplier()) * time.Second):
		t.Fatalf("Task unblock should have been called")
	}

	select {
	case <-harness.mockHooks.RestartCh:
		t.Fatalf("should not have restarted: %+v", harness.mockHooks)
	case <-harness.mockHooks.SignalCh:
		t.Fatalf("should not have restarted: %+v", harness.mockHooks)
	case <-time.After(time.Duration(1*testutil.TestMultiplier()) * time.Second):
	}

	// simulate a client restart and TTL expiry
	harness.manager.Stop()
	content = "bazbar"
	_, err = logical.Write(vaultPath, map[string]interface{}{"data": map[string]interface{}{key: content}})
	require.NoError(err)
	harness.mockHooks.UnblockCh = make(chan struct{}, 1)
	harness.start(t)

	// Wait for the unblock
	select {
	case <-harness.mockHooks.UnblockCh:
	case <-time.After(time.Duration(5*testutil.TestMultiplier()) * time.Second):
		t.Fatalf("Task unblock should have been called")
	}

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
}

func TestTaskTemplateManager_Rerender_Noop(t *testing.T) {
	t.Parallel()
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
	raw, err := ioutil.ReadFile(path)
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
	raw, err = ioutil.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read rendered template from %q: %v", path, err)
	}

	if s := string(raw); s != content2 {
		t.Fatalf("Unexpected template data; got %q, want %q", s, content2)
	}
}

func TestTaskTemplateManager_Rerender_Signal(t *testing.T) {
	t.Parallel()
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

	if len(harness.mockHooks.Signals) != 0 {
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
			harness.mockHooks.signalLock.Lock()
			s := harness.mockHooks.Signals
			harness.mockHooks.signalLock.Unlock()
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
	raw, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read rendered template from %q: %v", path, err)
	}

	if s := string(raw); s != content1_2 {
		t.Fatalf("Unexpected template data; got %q, want %q", s, content1_2)
	}

	path = filepath.Join(harness.taskDir, file2)
	raw, err = ioutil.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read rendered template from %q: %v", path, err)
	}

	if s := string(raw); s != content2_2 {
		t.Fatalf("Unexpected template data; got %q, want %q", s, content2_2)
	}
}

func TestTaskTemplateManager_Rerender_Restart(t *testing.T) {
	t.Parallel()
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
	raw, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read rendered template from %q: %v", path, err)
	}

	if s := string(raw); s != content1_2 {
		t.Fatalf("Unexpected template data; got %q, want %q", s, content1_2)
	}
}

func TestTaskTemplateManager_Interpolate_Destination(t *testing.T) {
	t.Parallel()
	// Make a template that will have its destination interpolated
	content := "hello, world!"
	file := "${node.unique.id}.tmpl"
	template := &structs.Template{
		EmbeddedTmpl: content,
		DestPath:     file,
		ChangeMode:   structs.TemplateChangeModeNoop,
	}

	harness := newTestHarness(t, []*structs.Template{template}, false, false)
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
	raw, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read rendered template from %q: %v", path, err)
	}

	if s := string(raw); s != content {
		t.Fatalf("Unexpected template data; got %q, want %q", s, content)
	}
}

func TestTaskTemplateManager_Signal_Error(t *testing.T) {
	t.Parallel()
	require := require.New(t)

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

	require.NotNil(harness.mockHooks.KillEvent)
	require.Contains(harness.mockHooks.KillEvent.DisplayMessage, "failed to send signals")
}

// TestTaskTemplateManager_FiltersProcessEnvVars asserts that we only render
// environment variables found in task env-vars and not read the nomad host
// process environment variables.  nomad host process environment variables
// are to be treated the same as not found environment variables.
func TestTaskTemplateManager_FiltersEnvVars(t *testing.T) {
	t.Parallel()

	defer os.Setenv("NOMAD_TASK_NAME", os.Getenv("NOMAD_TASK_NAME"))
	os.Setenv("NOMAD_TASK_NAME", "should be overridden by task")

	testenv := "TESTENV_" + strings.ReplaceAll(uuid.Generate(), "-", "")
	os.Setenv(testenv, "MY_TEST_VALUE")
	defer os.Unsetenv(testenv)

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
	harness.start(t)
	defer harness.stop()

	// Wait for the unblock
	select {
	case <-harness.mockHooks.UnblockCh:
	case <-time.After(time.Duration(5*testutil.TestMultiplier()) * time.Second):
		require.Fail(t, "Task unblock should have been called")
	}

	// Check the file is there
	path := filepath.Join(harness.taskDir, file)
	raw, err := ioutil.ReadFile(path)
	require.NoError(t, err)

	require.Equal(t, expected, string(raw))
}

// TestTaskTemplateManager_Env asserts templates with the env flag set are read
// into the task's environment.
func TestTaskTemplateManager_Env(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	d, err := ioutil.TempDir("", "ct_env_missing")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer os.RemoveAll(d)

	// Fake writing the file so we don't have to run the whole template manager
	err = ioutil.WriteFile(filepath.Join(d, "exists.env"), []byte("FOO=bar\n"), 0644)
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

	if vars, err := loadTemplateEnv(templates, d, taskenv.NewEmptyTaskEnv()); err == nil {
		t.Fatalf("expected an error but instead got env vars: %#v", vars)
	}
}

// TestTaskTemplateManager_Env_InterpolatedDest asserts the core env
// template processing function handles interpolated destinations
func TestTaskTemplateManager_Env_InterpolatedDest(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	d, err := ioutil.TempDir("", "ct_env_interpolated")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer os.RemoveAll(d)

	// Fake writing the file so we don't have to run the whole template manager
	err = ioutil.WriteFile(filepath.Join(d, "exists.env"), []byte("FOO=bar\n"), 0644)
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
		map[string]string{}, map[string]string{})

	vars, err := loadTemplateEnv(templates, d, taskEnv)
	require.NoError(err)
	require.Contains(vars, "FOO")
	require.Equal(vars["FOO"], "bar")
}

// TestTaskTemplateManager_Env_Multi asserts the core env
// template processing function returns combined env vars from multiple
// templates correctly.
func TestTaskTemplateManager_Env_Multi(t *testing.T) {
	t.Parallel()
	d, err := ioutil.TempDir("", "ct_env_missing")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer os.RemoveAll(d)

	// Fake writing the files so we don't have to run the whole template manager
	err = ioutil.WriteFile(filepath.Join(d, "zzz.env"), []byte("FOO=bar\nSHARED=nope\n"), 0644)
	if err != nil {
		t.Fatalf("error writing template file 1: %v", err)
	}
	err = ioutil.WriteFile(filepath.Join(d, "aaa.env"), []byte("BAR=foo\nSHARED=yup\n"), 0644)
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

	vars, err := loadTemplateEnv(templates, d, taskenv.NewEmptyTaskEnv())
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
	t.Parallel()
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
	t.Parallel()
	c := config.DefaultConfig()
	c.VaultConfig = &sconfig.VaultConfig{
		Enabled:       helper.BoolToPtr(true),
		Addr:          "https://localhost/",
		TLSServerName: "notlocalhost",
	}
	config := &TaskTemplateManagerConfig{
		ClientConfig: c,
		VaultToken:   "token",
	}
	ctconf, err := newRunnerConfig(config, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if *ctconf.Vault.SSL.ServerName != c.VaultConfig.TLSServerName {
		t.Fatalf("expected %q but found %q", c.VaultConfig.TLSServerName, *ctconf.Vault.SSL.ServerName)
	}
}

// TestTaskTemplateManager_Config_VaultNamespace asserts the Vault namespace setting is
// propagated to consul-template's configuration.
func TestTaskTemplateManager_Config_VaultNamespace(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	testNS := "test-namespace"
	c := config.DefaultConfig()
	c.Node = mock.Node()
	c.VaultConfig = &sconfig.VaultConfig{
		Enabled:       helper.BoolToPtr(true),
		Addr:          "https://localhost/",
		TLSServerName: "notlocalhost",
		Namespace:     testNS,
	}

	alloc := mock.Alloc()
	config := &TaskTemplateManagerConfig{
		ClientConfig: c,
		VaultToken:   "token",
		EnvBuilder:   taskenv.NewBuilder(c.Node, alloc, alloc.Job.TaskGroups[0].Tasks[0], c.Region),
	}

	ctmplMapping, err := parseTemplateConfigs(config)
	assert.Nil(err, "Parsing Templates")

	ctconf, err := newRunnerConfig(config, ctmplMapping)
	assert.Nil(err, "Building Runner Config")
	assert.Equal(testNS, *ctconf.Vault.Namespace, "Vault Namespace Value")
}

// TestTaskTemplateManager_Config_VaultNamespace asserts the Vault namespace setting is
// propagated to consul-template's configuration.
func TestTaskTemplateManager_Config_VaultNamespace_TaskOverride(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	testNS := "test-namespace"
	c := config.DefaultConfig()
	c.Node = mock.Node()
	c.VaultConfig = &sconfig.VaultConfig{
		Enabled:       helper.BoolToPtr(true),
		Addr:          "https://localhost/",
		TLSServerName: "notlocalhost",
		Namespace:     testNS,
	}

	alloc := mock.Alloc()
	overriddenNS := "new-namespace"

	// Set the template manager config vault namespace
	config := &TaskTemplateManagerConfig{
		ClientConfig:   c,
		VaultToken:     "token",
		VaultNamespace: overriddenNS,
		EnvBuilder:     taskenv.NewBuilder(c.Node, alloc, alloc.Job.TaskGroups[0].Tasks[0], c.Region),
	}

	ctmplMapping, err := parseTemplateConfigs(config)
	assert.Nil(err, "Parsing Templates")

	ctconf, err := newRunnerConfig(config, ctmplMapping)
	assert.Nil(err, "Building Runner Config")
	assert.Equal(overriddenNS, *ctconf.Vault.Namespace, "Vault Namespace Value")
}

// TestTaskTemplateManager_Escapes asserts that when sandboxing is enabled
// interpolated paths are not incorrectly treated as escaping the alloc dir.
func TestTaskTemplateManager_Escapes(t *testing.T) {
	t.Parallel()

	clientConf := config.DefaultConfig()
	require.False(t, clientConf.TemplateConfig.DisableSandbox, "expected sandbox to be disabled")

	// Set a fake alloc dir to make test output more realistic
	clientConf.AllocDir = "/fake/allocdir"

	clientConf.Node = mock.Node()
	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	logger := testlog.HCLogger(t)
	allocDir := allocdir.NewAllocDir(logger, filepath.Join(clientConf.AllocDir, alloc.ID))
	taskDir := allocDir.NewTaskDir(task.Name)

	containerEnv := func() *taskenv.Builder {
		// To emulate a Docker or exec tasks we must copy the
		// Set{Alloc,Task,Secrets}Dir logic in taskrunner/task_dir_hook.go
		b := taskenv.NewBuilder(clientConf.Node, alloc, task, clientConf.Region)
		b.SetAllocDir(allocdir.SharedAllocContainerPath)
		b.SetTaskLocalDir(allocdir.TaskLocalContainerPath)
		b.SetSecretsDir(allocdir.TaskSecretsContainerPath)
		return b
	}

	rawExecEnv := func() *taskenv.Builder {
		// To emulate a unisolated tasks we must copy the
		// Set{Alloc,Task,Secrets}Dir logic in taskrunner/task_dir_hook.go
		b := taskenv.NewBuilder(clientConf.Node, alloc, task, clientConf.Region)
		b.SetAllocDir(taskDir.SharedAllocDir)
		b.SetTaskLocalDir(taskDir.LocalDir)
		b.SetSecretsDir(taskDir.SecretsDir)
		return b
	}

	cases := []struct {
		Name   string
		Config func() *TaskTemplateManagerConfig

		// Set to skip a test; remove once bugs are fixed
		Skip bool

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
			Skip: true,
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
			if tc.Skip {
				t.Skip("FIXME: Skipping broken test")
			}
			config := tc.Config()
			mapping, err := parseTemplateConfigs(config)
			if tc.Err == nil {
				// Ok path
				require.NoError(t, err)
				require.NotNil(t, mapping)
				require.Len(t, mapping, 1)
				for k := range mapping {
					require.Equal(t, tc.SourcePath, *k.Source)
					require.Equal(t, tc.DestPath, *k.Destination)
					t.Logf("Rendering %s => %s", *k.Source, *k.Destination)
				}
			} else {
				// Err path
				assert.EqualError(t, err, tc.Err.Error())
				require.Nil(t, mapping)
			}

		})
	}
}

func TestTaskTemplateManager_BlockedEvents(t *testing.T) {
	// The tests sets a template that need keys 0, 1, 2, 3, 4,
	// then subsequently sets 0, 1, 2 keys
	// then asserts that templates are still blocked on 3 and 4,
	// and check that we got the relevant task events
	t.Parallel()
	require := require.New(t)

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
	require.Len(harness.mockHooks.Events, 1)
	t.Logf("first message: %v", harness.mockHooks.Events[0])
	missing, more := missingKeys(harness.mockHooks.Events[0])
	require.Equal(5, len(missing)+more)
	require.Contains(harness.mockHooks.Events[0].DisplayMessage, "and 2 more")

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
	event := harness.mockHooks.Events[len(harness.mockHooks.Events)-1]
	if !isExpectedFinalEvent(event) {
		t.Logf("received all events: %v", pretty.Sprint(harness.mockHooks.Events))

		t.Fatalf("bad event, expected only 3 and 5 blocked got: %q", event.DisplayMessage)
	}
}
