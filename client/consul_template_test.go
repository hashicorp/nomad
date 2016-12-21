package client

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	ctestutil "github.com/hashicorp/consul/testutil"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/driver/env"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	sconfig "github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/hashicorp/nomad/testutil"
)

// MockTaskHooks is a mock of the TaskHooks interface useful for testing
type MockTaskHooks struct {
	Restarts  int
	RestartCh chan struct{}

	Signals  []os.Signal
	SignalCh chan struct{}

	// SignalError is returned when Signal is called on the mock hook
	SignalError error

	UnblockCh chan struct{}
	Unblocked bool

	KillReason string
	KillCh     chan struct{}
}

func NewMockTaskHooks() *MockTaskHooks {
	return &MockTaskHooks{
		UnblockCh: make(chan struct{}, 1),
		RestartCh: make(chan struct{}, 1),
		SignalCh:  make(chan struct{}, 1),
		KillCh:    make(chan struct{}, 1),
	}
}
func (m *MockTaskHooks) Restart(source, reason string) {
	m.Restarts++
	select {
	case m.RestartCh <- struct{}{}:
	default:
	}
}

func (m *MockTaskHooks) Signal(source, reason string, s os.Signal) error {
	m.Signals = append(m.Signals, s)
	select {
	case m.SignalCh <- struct{}{}:
	default:
	}

	return m.SignalError
}

func (m *MockTaskHooks) Kill(source, reason string, fail bool) {
	m.KillReason = reason
	select {
	case m.KillCh <- struct{}{}:
	default:
	}
}

func (m *MockTaskHooks) UnblockStart(source string) {
	if !m.Unblocked {
		close(m.UnblockCh)
	}

	m.Unblocked = true
}

// testHarness is used to test the TaskTemplateManager by spinning up
// Consul/Vault as needed
type testHarness struct {
	manager    *TaskTemplateManager
	mockHooks  *MockTaskHooks
	templates  []*structs.Template
	taskEnv    *env.TaskEnvironment
	node       *structs.Node
	config     *config.Config
	vaultToken string
	taskDir    string
	vault      *testutil.TestVault
	consul     *ctestutil.TestServer
}

// newTestHarness returns a harness starting a dev consul and vault server,
// building the appropriate config and creating a TaskTemplateManager
func newTestHarness(t *testing.T, templates []*structs.Template, consul, vault bool) *testHarness {
	harness := &testHarness{
		mockHooks: NewMockTaskHooks(),
		templates: templates,
		node:      mock.Node(),
		config:    &config.Config{},
	}

	// Build the task environment
	harness.taskEnv = env.NewTaskEnvironment(harness.node)

	// Make a tempdir
	d, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("Failed to make tmpdir: %v", err)
	}
	harness.taskDir = d

	if consul {
		harness.consul = ctestutil.NewTestServer(t)
		harness.config.ConsulConfig = &sconfig.ConsulConfig{
			Addr: harness.consul.HTTPAddr,
		}
	}

	if vault {
		harness.vault = testutil.NewTestVault(t).Start()
		harness.config.VaultConfig = harness.vault.Config
		harness.vaultToken = harness.vault.RootToken
	}

	return harness
}

func (h *testHarness) start(t *testing.T) {
	manager, err := NewTaskTemplateManager(h.mockHooks, h.templates,
		h.config, h.vaultToken, h.taskDir, h.taskEnv)
	if err != nil {
		t.Fatalf("failed to build task template manager: %v", err)
	}

	h.manager = manager
}

func (h *testHarness) startWithErr() error {
	manager, err := NewTaskTemplateManager(h.mockHooks, h.templates,
		h.config, h.vaultToken, h.taskDir, h.taskEnv)
	h.manager = manager
	return err
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

func TestTaskTemplateManager_Invalid(t *testing.T) {
	hooks := NewMockTaskHooks()
	var tmpls []*structs.Template
	config := &config.Config{}
	taskDir := "foo"
	vaultToken := ""
	taskEnv := env.NewTaskEnvironment(mock.Node())

	_, err := NewTaskTemplateManager(nil, nil, nil, "", "", nil)
	if err == nil {
		t.Fatalf("Expected error")
	}

	_, err = NewTaskTemplateManager(nil, tmpls, config, vaultToken, taskDir, taskEnv)
	if err == nil || !strings.Contains(err.Error(), "task hook") {
		t.Fatalf("Expected invalid task hook error: %v", err)
	}

	_, err = NewTaskTemplateManager(hooks, tmpls, nil, vaultToken, taskDir, taskEnv)
	if err == nil || !strings.Contains(err.Error(), "config") {
		t.Fatalf("Expected invalid config error: %v", err)
	}

	_, err = NewTaskTemplateManager(hooks, tmpls, config, vaultToken, "", taskEnv)
	if err == nil || !strings.Contains(err.Error(), "task directory") {
		t.Fatalf("Expected invalid task dir error: %v", err)
	}

	_, err = NewTaskTemplateManager(hooks, tmpls, config, vaultToken, taskDir, nil)
	if err == nil || !strings.Contains(err.Error(), "task environment") {
		t.Fatalf("Expected invalid task environment error: %v", err)
	}

	tm, err := NewTaskTemplateManager(hooks, tmpls, config, vaultToken, taskDir, taskEnv)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	} else if tm == nil {
		t.Fatalf("Bad %v", tm)
	}

	// Build a template with a bad signal
	tmpl := &structs.Template{
		DestPath:     "foo",
		EmbeddedTmpl: "hello, world",
		ChangeMode:   structs.TemplateChangeModeSignal,
		ChangeSignal: "foobarbaz",
	}

	tmpls = append(tmpls, tmpl)
	tm, err = NewTaskTemplateManager(hooks, tmpls, config, vaultToken, taskDir, taskEnv)
	if err == nil || !strings.Contains(err.Error(), "Failed to parse signal") {
		t.Fatalf("Expected signal parsing error: %v", err)
	}
}

func TestTaskTemplateManager_HostPath(t *testing.T) {
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

	// Change the config to disallow host sources
	harness = newTestHarness(t, []*structs.Template{template}, false, false)
	harness.config.Options = map[string]string{
		hostSrcOption: "false",
	}
	if err := harness.startWithErr(); err == nil || !strings.Contains(err.Error(), "absolute") {
		t.Fatalf("Expected absolute template path disallowed: %v", err)
	}
}

func TestTaskTemplateManager_Unblock_Static(t *testing.T) {
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

func TestTaskTemplateManager_Unblock_Static_AlreadyRendered(t *testing.T) {
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

	// Drop the retry rate
	testRetryRate = 10 * time.Millisecond

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
	harness.consul.SetKV(key, []byte(content))

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
	// Make a template that will render based on a key in Vault
	vaultPath := "secret/password"
	key := "password"
	content := "barbaz"
	embedded := fmt.Sprintf(`{{with secret "%s"}}{{.Data.%s}}{{end}}`, vaultPath, key)
	file := "my.tmpl"
	template := &structs.Template{
		EmbeddedTmpl: embedded,
		DestPath:     file,
		ChangeMode:   structs.TemplateChangeModeNoop,
	}

	// Drop the retry rate
	testRetryRate = 10 * time.Millisecond

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
	logical.Write(vaultPath, map[string]interface{}{key: content})

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

	// Drop the retry rate
	testRetryRate = 10 * time.Millisecond

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
	harness.consul.SetKV(consulKey, []byte(consulContent))

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

func TestTaskTemplateManager_Rerender_Noop(t *testing.T) {
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

	// Drop the retry rate
	testRetryRate = 10 * time.Millisecond

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
	harness.consul.SetKV(key, []byte(content1))

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
	harness.consul.SetKV(key, []byte(content2))

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

	// Drop the retry rate
	testRetryRate = 10 * time.Millisecond

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
	harness.consul.SetKV(key1, []byte(content1_1))
	harness.consul.SetKV(key2, []byte(content2_1))

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
	harness.consul.SetKV(key1, []byte(content1_2))
	harness.consul.SetKV(key2, []byte(content2_2))

	// Wait for signals
	timeout := time.After(time.Duration(1*testutil.TestMultiplier()) * time.Second)
OUTER:
	for {
		select {
		case <-harness.mockHooks.RestartCh:
			t.Fatalf("Restart with signal policy: %+v", harness.mockHooks)
		case <-harness.mockHooks.SignalCh:
			if len(harness.mockHooks.Signals) != 2 {
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

	// Drop the retry rate
	testRetryRate = 10 * time.Millisecond

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
	harness.consul.SetKV(key1, []byte(content1_1))

	// Wait for the unblock
	select {
	case <-harness.mockHooks.UnblockCh:
	case <-time.After(time.Duration(5*testutil.TestMultiplier()) * time.Second):
		t.Fatalf("Task unblock should have been called")
	}

	// Update the keys in Consul
	harness.consul.SetKV(key1, []byte(content1_2))

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

	// Drop the retry rate
	testRetryRate = 10 * time.Millisecond

	harness := newTestHarness(t, []*structs.Template{template}, true, false)
	harness.start(t)
	defer harness.stop()

	harness.mockHooks.SignalError = fmt.Errorf("test error")

	// Write the key to Consul
	harness.consul.SetKV(key1, []byte(content1))

	// Wait a little
	select {
	case <-harness.mockHooks.UnblockCh:
	case <-time.After(time.Duration(2*testutil.TestMultiplier()) * time.Second):
		t.Fatalf("Should have received unblock: %+v", harness.mockHooks)
	}

	// Write the key to Consul
	harness.consul.SetKV(key1, []byte(content2))

	// Wait for kill channel
	select {
	case <-harness.mockHooks.KillCh:
		break
	case <-time.After(time.Duration(1*testutil.TestMultiplier()) * time.Second):
		t.Fatalf("Should have received a signals: %+v", harness.mockHooks)
	}

	if !strings.Contains(harness.mockHooks.KillReason, "Sending signals") {
		t.Fatalf("Unexpected error", harness.mockHooks.KillReason)
	}
}
