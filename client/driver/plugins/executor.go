package plugins

import (
	"net/rpc"
	"os/exec"
	"time"

	"github.com/hashicorp/go-plugin"
)

var HandshakeConfig = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "executor_plugin",
	MagicCookieValue: "value",
}

var PluginMap = map[string]plugin.Plugin{
	"executor": new(ExecutorPlugin),
}

type ExecutorContext struct {
}

type ProcessState struct {
	Pid      int
	ExitCode int
	Time     time.Time
}

type Executor interface {
	LaunchCmd(cmd *exec.Cmd, ctx *ExecutorContext) (*ProcessState, error)
	Wait() (*ProcessState, error)
	ShutDown() (*ProcessState, error)
	Exit() (*ProcessState, error)
}

type ExecutorRPC struct {
	client *rpc.Client
}

type LaunchCmdArgs struct {
	Cmd *exec.Cmd
	Ctx *ExecutorContext
}

func (e *ExecutorRPC) LaunchCmd(cmd *exec.Cmd, ctx *ExecutorContext) (*ProcessState, error) {
	var ps ProcessState
	err := e.client.Call("Plugin.LaunchCmd", LaunchCmdArgs{Cmd: cmd, Ctx: ctx}, &ps)
	return &ps, err
}

func (e *ExecutorRPC) Wait() (*ProcessState, error) {
	var ps ProcessState
	err := e.client.Call("Plugin.Wait", new(interface{}), &ps)
	return &ps, err
}

func (e *ExecutorRPC) ShutDown() (*ProcessState, error) {
	var ps ProcessState
	err := e.client.Call("Plugin.ShutDown", new(interface{}), &ps)
	return &ps, err
}

func (e *ExecutorRPC) Exit() (*ProcessState, error) {
	var ps ProcessState
	err := e.client.Call("Plugin.Exit", new(interface{}), &ps)
	return &ps, err
}

type ExecutorRPCServer struct {
	Impl Executor
}

func (e *ExecutorRPCServer) LaunchCmd(args LaunchCmdArgs, ps *ProcessState) error {
	var err error
	ps, err = e.Impl.LaunchCmd(args.Cmd, args.Ctx)
	return err
}

func (e *ExecutorRPCServer) Wait(args interface{}, ps *ProcessState) error {
	var err error
	ps, err = e.Impl.Wait()
	return err
}

func (e *ExecutorRPCServer) ShutDown(args interface{}, ps *ProcessState) error {
	var err error
	ps, err = e.Impl.ShutDown()
	return err
}

func (e *ExecutorRPCServer) Exit(args interface{}, ps *ProcessState) error {
	var err error
	ps, err = e.Impl.Exit()
	return err
}

type ExecutorPlugin struct{}

func (p *ExecutorPlugin) Server(*plugin.MuxBroker) (interface{}, error) {
	return &ExecutorRPCServer{Impl: NewExecutor()}, nil
}

func (p *ExecutorPlugin) Client(b *plugin.MuxBroker, c *rpc.Client) (interface{}, error) {
	return &ExecutorRPC{client: c}, nil
}
