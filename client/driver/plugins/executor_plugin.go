package plugins

import (
	"fmt"
	"log"
	"net/rpc"
	"os"

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

type ExecutorRPC struct {
	client *rpc.Client
}

type LaunchCmdArgs struct {
	Cmd *ExecCommand
	Ctx *ExecutorContext
}

func (e *ExecutorRPC) LaunchCmd(cmd *ExecCommand, ctx *ExecutorContext) (*ProcessState, error) {
	ps := new(ProcessState)
	err := e.client.Call("Plugin.LaunchCmd", LaunchCmdArgs{Cmd: cmd, Ctx: ctx}, ps)
	return ps, err
}

func (e *ExecutorRPC) Wait() (*ProcessState, error) {
	var ps ProcessState
	err := e.client.Call("Plugin.Wait", new(interface{}), &ps)
	return &ps, err
}

func (e *ExecutorRPC) ShutDown() error {
	var ps ProcessState
	err := e.client.Call("Plugin.ShutDown", new(interface{}), &ps)
	return err
}

func (e *ExecutorRPC) Exit() error {
	var ps ProcessState
	err := e.client.Call("Plugin.Exit", new(interface{}), &ps)
	return err
}

type ExecutorRPCServer struct {
	Impl Executor
}

func (e *ExecutorRPCServer) LaunchCmd(args LaunchCmdArgs, ps *ProcessState) error {
	var err error
	ps, err = e.Impl.LaunchCmd(args.Cmd, args.Ctx)
	fmt.Printf("DIPTANU PS Server : %#v", ps)
	return err
}

func (e *ExecutorRPCServer) Wait(args interface{}, ps *ProcessState) error {
	var err error
	ps, err = e.Impl.Wait()
	return err
}

func (e *ExecutorRPCServer) ShutDown(args interface{}, ps *ProcessState) error {
	var err error
	err = e.Impl.ShutDown()
	return err
}

func (e *ExecutorRPCServer) Exit(args interface{}, ps *ProcessState) error {
	var err error
	err = e.Impl.Exit()
	return err
}

type ExecutorPlugin struct {
	logger *log.Logger
}

func (p *ExecutorPlugin) Server(*plugin.MuxBroker) (interface{}, error) {
	p.logger = log.New(os.Stdout, "executor-plugin-server:", log.LstdFlags)
	return &ExecutorRPCServer{Impl: NewExecutor(p.logger)}, nil
}

func (p *ExecutorPlugin) Client(b *plugin.MuxBroker, c *rpc.Client) (interface{}, error) {
	p.logger = log.New(os.Stdout, "executor-plugin-client:", log.LstdFlags)
	return &ExecutorRPC{client: c}, nil
}
