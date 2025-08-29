// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package execagent

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/api"
)

type AgentMode int

const (
	// Conf enum is for configuring either a client, server, or mixed agent.
	ModeClient AgentMode = 1
	ModeServer AgentMode = 2
	ModeBoth             = ModeClient | ModeServer
)

func init() {
	if d := os.Getenv("NOMAD_TEST_DIR"); d != "" {
		BaseDir = d
	}
}

var (
	// BaseDir is where tests will store state and can be overridden by
	// setting NOMAD_TEST_DIR. Defaults to "/opt/nomadtest"
	BaseDir = "/opt/nomadtest"

	agentTemplate = template.Must(template.New("agent").Parse(`
enable_debug = true
name         = "{{ or .AgentName "nomad-e2e-test-agent" }}"
log_level    = "{{ or .LogLevel "DEBUG" }}"

ports {
  http = {{.HTTP}}
  rpc  = {{.RPC}}
  serf = {{.Serf}}
}

{{ if .EnableServer }}
server {
  enabled = true
  bootstrap_expect = 1
}
{{ end }}

{{ if .EnableClient }}
client {
  enabled   = true
  node_pool = "{{ or .NodePool "default" }}"

  options = {
    "driver.raw_exec.enable" = "1"
  }
  {{- $retry_join_length := len .RetryJoinAddrs }}{{ if not (eq $retry_join_length 0) }}
  server_join {
    retry_join = [{{ range $index, $element := .RetryJoinAddrs }}{{if $index}}, {{end}}"{{$element}}"{{ end }}]
  }
  {{ end }}
}
{{ end }}
`))
)

type AgentTemplateVars struct {
	HTTP         int
	RPC          int
	Serf         int
	EnableClient bool
	EnableServer bool

	// AgentName is the name to apply to the Nomad agent. This is optional, but
	// allows for multiple agents to be run on the same host. If not set, it
	// will default to "nomad-e2e-test-agent".
	AgentName string

	LogLevel string

	// NodePool is the Nomad node pool to assign the agent to when running with
	// client mode enabled. This will default to the "default" node pool if not
	// set.
	NodePool string

	// RetryJoinAddrs is a list of addresses to use for the retry_join config
	// block.
	RetryJoinAddrs []string
}

func newAgentTemplateVars() (*AgentTemplateVars, error) {
	httpPort, err := getFreePort()
	if err != nil {
		return nil, err
	}
	rpcPort, err := getFreePort()
	if err != nil {
		return nil, err
	}
	serfPort, err := getFreePort()
	if err != nil {
		return nil, err
	}

	vars := AgentTemplateVars{
		HTTP:     httpPort,
		RPC:      rpcPort,
		Serf:     serfPort,
		LogLevel: hclog.Warn.String(),
		NodePool: "default",
	}

	return &vars, nil
}

// SetMode is a helper function to allow setting the agent mode (client, server,
// or both).
func (a *AgentTemplateVars) SetMode(mode AgentMode) {
	switch mode {
	case ModeClient:
		a.EnableClient = true
		a.EnableServer = false
	case ModeServer:
		a.EnableClient = false
		a.EnableServer = true
	case ModeBoth:
		a.EnableClient = true
		a.EnableServer = true
	}
}

func writeConfig(path string, vars *AgentTemplateVars) error {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	return agentTemplate.Execute(f, vars)
}

// NomadAgent manages an external Nomad agent process.
type NomadAgent struct {
	// BinPath is the path to the Nomad binary
	BinPath string

	// DataDir is the path state will be saved in
	DataDir string

	// ConfFile is the path to the agent's conf file
	ConfFile string

	// Cmd is the agent process
	Cmd *exec.Cmd

	// Vars are the config parameters used to template
	Vars *AgentTemplateVars
}

// NewMixedAgent creates a new Nomad agent in mixed server+client mode but does
// not start the agent process until the Start() method is called.
func NewMixedAgent(bin string) (*NomadAgent, error) {
	if err := os.MkdirAll(BaseDir, 0755); err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp(BaseDir, "agent")
	if err != nil {
		return nil, err
	}

	vars, err := newAgentTemplateVars()
	if err != nil {
		return nil, err
	}
	vars.EnableClient = true
	vars.EnableServer = true

	conf := filepath.Join(dir, "config.hcl")
	if err := writeConfig(conf, vars); err != nil {
		return nil, err
	}

	na := &NomadAgent{
		BinPath:  bin,
		DataDir:  dir,
		ConfFile: conf,
		Vars:     vars,
		Cmd:      exec.Command(bin, "agent", "-config", conf, "-data-dir", dir),
	}
	return na, nil
}

// NewClientServerPair creates a pair of Nomad agents: 1 server, 1 client.
func NewClientServerPair(bin string, serverOut, clientOut io.Writer) (
	server *NomadAgent, client *NomadAgent, err error) {

	if err := os.MkdirAll(BaseDir, 0755); err != nil {
		return nil, nil, err
	}

	sdir, err := os.MkdirTemp(BaseDir, "server")
	if err != nil {
		return nil, nil, err
	}

	svars, err := newAgentTemplateVars()
	if err != nil {
		return nil, nil, err
	}
	svars.LogLevel = "WARN"
	svars.EnableServer = true

	sconf := filepath.Join(sdir, "config.hcl")
	if err := writeConfig(sconf, svars); err != nil {
		return nil, nil, err
	}

	server = &NomadAgent{
		BinPath:  bin,
		DataDir:  sdir,
		ConfFile: sconf,
		Vars:     svars,
		Cmd:      exec.Command(bin, "agent", "-config", sconf, "-data-dir", sdir),
	}
	server.Cmd.Stdout = serverOut
	server.Cmd.Stderr = serverOut

	cdir, err := os.MkdirTemp(BaseDir, "client")
	if err != nil {
		return nil, nil, err
	}

	cvars, err := newAgentTemplateVars()
	if err != nil {
		return nil, nil, err
	}
	cvars.EnableClient = true

	cconf := filepath.Join(cdir, "config.hcl")
	if err := writeConfig(cconf, cvars); err != nil {
		return nil, nil, err
	}

	client = &NomadAgent{
		BinPath:  bin,
		DataDir:  cdir,
		ConfFile: cconf,
		Vars:     cvars,
		Cmd: exec.Command(bin, "agent",
			"-config", cconf,
			"-data-dir", cdir,
			"-servers", fmt.Sprintf("127.0.0.1:%d", svars.RPC),
		),
	}
	client.Cmd.Stdout = clientOut
	client.Cmd.Stderr = clientOut
	return
}

// TemplateVariableCallbackFunc is a callback function that allow callers to
// modify the template variables before the config file is written out.
type TemplateVariableCallbackFunc func(c *AgentTemplateVars)

func NewSingleModeAgent(
	bin, baseDir, additionalConfig string,
	mode AgentMode,
	writer io.Writer,
	varCallbackFn TemplateVariableCallbackFunc,
) (*NomadAgent, error) {

	templateVars, err := newAgentTemplateVars()
	if err != nil {
		return nil, err
	}

	// Allow the caller to modify the template variables before we write out the
	// config file.
	if varCallbackFn != nil {
		varCallbackFn(templateVars)
	}

	// Set the mode (client, server, or both)
	templateVars.SetMode(mode)

	baseDataDir := BaseDir

	if baseDir != "" {
		baseDataDir = baseDir
	}

	if err := os.MkdirAll(baseDataDir, 0755); err != nil {
		return nil, err
	}

	agentDir, err := os.MkdirTemp(baseDataDir, "agent")
	if err != nil {
		return nil, err
	}

	agentConfig := filepath.Join(agentDir, "agent.hcl")
	if err := writeConfig(agentConfig, templateVars); err != nil {
		return nil, err
	}

	commandArgs := []string{
		"agent",
		"-config=" + agentConfig,
		"-data-dir=" + agentDir,
	}

	// If the caller specifieed additional config, write it out to a file and
	// add it to the command args.
	//
	// This allows for arbitrary config to be added that isn't supported by the
	// template.
	//
	// The caller is responsible for ensuring the additional config is valid.
	if additionalConfig != "" {

		extraFilePath := filepath.Join(agentDir, "extra.hcl")

		if err := os.WriteFile(extraFilePath, []byte(additionalConfig), 0755); err != nil {
			return nil, err
		}

		commandArgs = append(commandArgs, "-config="+extraFilePath)
	}

	nomadAgent := &NomadAgent{
		BinPath:  bin,
		DataDir:  agentDir,
		ConfFile: agentConfig,
		Vars:     templateVars,
		Cmd:      exec.Command(bin, commandArgs...),
	}

	nomadAgent.Cmd.Stdout = writer
	nomadAgent.Cmd.Stderr = writer

	return nomadAgent, nil
}

// Start the agent command.
func (n *NomadAgent) Start() error {
	return n.Cmd.Start()
}

// Stop sends an interrupt signal and returns the command's Wait error.
func (n *NomadAgent) Stop() error {
	if err := n.Cmd.Process.Signal(os.Interrupt); err != nil {
		return err
	}

	return n.Cmd.Wait()
}

// Destroy stops the agent and removes the data dir.
func (n *NomadAgent) Destroy() error {
	if err := n.Stop(); err != nil {
		return err
	}
	return os.RemoveAll(n.DataDir)
}

// Client returns an api.Client for the agent.
func (n *NomadAgent) Client() (*api.Client, error) {
	conf := api.DefaultConfig()
	conf.Address = fmt.Sprintf("http://127.0.0.1:%d", n.Vars.HTTP)
	return api.NewClient(conf)
}

func getFreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}
