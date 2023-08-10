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
log_level = "{{ or .LogLevel "DEBUG" }}"

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
  enabled = true
  options = {
    "driver.raw_exec.enable" = "1"
  }
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
	LogLevel     string
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
		HTTP: httpPort,
		RPC:  rpcPort,
		Serf: serfPort,
	}

	return &vars, nil
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
