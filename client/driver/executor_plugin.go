package driver

import (
	"io"
	"log"
	"net"
	"net/rpc"

	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/client/driver/executor"
)

var HandshakeConfig = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "NOMAD_PLUGIN_MAGIC_COOKIE",
	MagicCookieValue: "e4327c2e01eabfd75a8a67adb114fb34a757d57eee7728d857a8cec6e91a7255",
}

func GetPluginMap(w io.Writer) map[string]plugin.Plugin {
	p := new(ExecutorPlugin)
	p.logger = log.New(w, "executor-plugin-server:", log.LstdFlags)
	return map[string]plugin.Plugin{"executor": p}
}

// ExecutorReattachConfig is the config that we seralize and de-serialize and
// store in disk
type ExecutorReattachConfig struct {
	Pid      int
	AddrNet  string
	AddrName string
}

// PluginConfig returns a config from an ExecutorReattachConfig
func (c *ExecutorReattachConfig) PluginConfig() *plugin.ReattachConfig {
	var addr net.Addr
	switch c.AddrNet {
	case "unix", "unixgram", "unixpacket":
		addr, _ = net.ResolveUnixAddr(c.AddrNet, c.AddrName)
	case "tcp", "tcp4", "tcp6":
		addr, _ = net.ResolveTCPAddr(c.AddrNet, c.AddrName)
	}
	return &plugin.ReattachConfig{Pid: c.Pid, Addr: addr}
}

func NewExecutorReattachConfig(c *plugin.ReattachConfig) *ExecutorReattachConfig {
	return &ExecutorReattachConfig{Pid: c.Pid, AddrNet: c.Addr.Network(), AddrName: c.Addr.String()}
}

type ExecutorRPC struct {
	client *rpc.Client
}

// LaunchCmdArgs wraps a user command and the args for the purposes of RPC
type LaunchCmdArgs struct {
	Cmd *executor.ExecCommand
	Ctx *executor.ExecutorContext
}

func (e *ExecutorRPC) LaunchCmd(cmd *executor.ExecCommand, ctx *executor.ExecutorContext) (*executor.ProcessState, error) {
	var ps *executor.ProcessState
	err := e.client.Call("Plugin.LaunchCmd", LaunchCmdArgs{Cmd: cmd, Ctx: ctx}, &ps)
	return ps, err
}

func (e *ExecutorRPC) Wait() (*executor.ProcessState, error) {
	var ps executor.ProcessState
	err := e.client.Call("Plugin.Wait", new(interface{}), &ps)
	return &ps, err
}

func (e *ExecutorRPC) ShutDown() error {
	return e.client.Call("Plugin.ShutDown", new(interface{}), new(interface{}))
}

func (e *ExecutorRPC) Exit() error {
	return e.client.Call("Plugin.Exit", new(interface{}), new(interface{}))
}

type ExecutorRPCServer struct {
	Impl executor.Executor
}

func (e *ExecutorRPCServer) LaunchCmd(args LaunchCmdArgs, ps *executor.ProcessState) error {
	state, err := e.Impl.LaunchCmd(args.Cmd, args.Ctx)
	if state != nil {
		*ps = *state
	}
	return err
}

func (e *ExecutorRPCServer) Wait(args interface{}, ps *executor.ProcessState) error {
	state, err := e.Impl.Wait()
	if state != nil {
		*ps = *state
	}
	return err
}

func (e *ExecutorRPCServer) ShutDown(args interface{}, resp *interface{}) error {
	return e.Impl.ShutDown()
}

func (e *ExecutorRPCServer) Exit(args interface{}, resp *interface{}) error {
	return e.Impl.Exit()
}

type ExecutorPlugin struct {
	logger *log.Logger
}

func (p *ExecutorPlugin) Server(*plugin.MuxBroker) (interface{}, error) {
	return &ExecutorRPCServer{Impl: executor.NewExecutor(p.logger)}, nil
}

func (p *ExecutorPlugin) Client(b *plugin.MuxBroker, c *rpc.Client) (interface{}, error) {
	return &ExecutorRPC{client: c}, nil
}
