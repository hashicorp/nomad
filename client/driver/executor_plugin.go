package driver

import (
	"encoding/gob"
	"log"
	"net/rpc"
	"os"
	"syscall"
	"time"

	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/client/driver/executor"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/structs"
)

// Registering these types since we have to serialize and de-serialize the Task
// structs over the wire between drivers and the executor.
func init() {
	gob.Register([]interface{}{})
	gob.Register(map[string]interface{}{})
	gob.Register([]map[string]string{})
	gob.Register([]map[string]int{})
	gob.Register(syscall.Signal(0x1))
}

type ExecutorRPC struct {
	client *rpc.Client
	logger *log.Logger
}

// LaunchCmdArgs wraps a user command and the args for the purposes of RPC
type LaunchCmdArgs struct {
	Cmd *executor.ExecCommand
}

type ExecCmdArgs struct {
	Deadline time.Time
	Name     string
	Args     []string
}

type ExecCmdReturn struct {
	Output []byte
	Code   int
}

func (e *ExecutorRPC) LaunchCmd(cmd *executor.ExecCommand) (*executor.ProcessState, error) {
	var ps *executor.ProcessState
	err := e.client.Call("Plugin.LaunchCmd", LaunchCmdArgs{Cmd: cmd}, &ps)
	return ps, err
}

func (e *ExecutorRPC) LaunchSyslogServer() (*executor.SyslogServerState, error) {
	var ss *executor.SyslogServerState
	err := e.client.Call("Plugin.LaunchSyslogServer", new(interface{}), &ss)
	return ss, err
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

func (e *ExecutorRPC) SetContext(ctx *executor.ExecutorContext) error {
	return e.client.Call("Plugin.SetContext", ctx, new(interface{}))
}

func (e *ExecutorRPC) UpdateLogConfig(logConfig *structs.LogConfig) error {
	return e.client.Call("Plugin.UpdateLogConfig", logConfig, new(interface{}))
}

func (e *ExecutorRPC) UpdateTask(task *structs.Task) error {
	return e.client.Call("Plugin.UpdateTask", task, new(interface{}))
}

func (e *ExecutorRPC) DeregisterServices() error {
	return e.client.Call("Plugin.DeregisterServices", new(interface{}), new(interface{}))
}

func (e *ExecutorRPC) Version() (*executor.ExecutorVersion, error) {
	var version executor.ExecutorVersion
	err := e.client.Call("Plugin.Version", new(interface{}), &version)
	return &version, err
}

func (e *ExecutorRPC) Stats() (*cstructs.TaskResourceUsage, error) {
	var resourceUsage cstructs.TaskResourceUsage
	err := e.client.Call("Plugin.Stats", new(interface{}), &resourceUsage)
	return &resourceUsage, err
}

func (e *ExecutorRPC) Signal(s os.Signal) error {
	return e.client.Call("Plugin.Signal", &s, new(interface{}))
}

func (e *ExecutorRPC) Exec(deadline time.Time, name string, args []string) ([]byte, int, error) {
	req := ExecCmdArgs{
		Deadline: deadline,
		Name:     name,
		Args:     args,
	}
	var resp *ExecCmdReturn
	err := e.client.Call("Plugin.Exec", req, &resp)
	if resp == nil {
		return nil, 0, err
	}
	return resp.Output, resp.Code, err
}

type ExecutorRPCServer struct {
	Impl   executor.Executor
	logger *log.Logger
}

func (e *ExecutorRPCServer) LaunchCmd(args LaunchCmdArgs, ps *executor.ProcessState) error {
	state, err := e.Impl.LaunchCmd(args.Cmd)
	if state != nil {
		*ps = *state
	}
	return err
}

func (e *ExecutorRPCServer) LaunchSyslogServer(args interface{}, ss *executor.SyslogServerState) error {
	state, err := e.Impl.LaunchSyslogServer()
	if state != nil {
		*ss = *state
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

func (e *ExecutorRPCServer) SetContext(args *executor.ExecutorContext, resp *interface{}) error {
	return e.Impl.SetContext(args)
}

func (e *ExecutorRPCServer) UpdateLogConfig(args *structs.LogConfig, resp *interface{}) error {
	return e.Impl.UpdateLogConfig(args)
}

func (e *ExecutorRPCServer) UpdateTask(args *structs.Task, resp *interface{}) error {
	return e.Impl.UpdateTask(args)
}

func (e *ExecutorRPCServer) DeregisterServices(args interface{}, resp *interface{}) error {
	// In 0.6 this is a noop. Goes away in 0.7.
	return nil
}

func (e *ExecutorRPCServer) Version(args interface{}, version *executor.ExecutorVersion) error {
	ver, err := e.Impl.Version()
	if ver != nil {
		*version = *ver
	}
	return err
}

func (e *ExecutorRPCServer) Stats(args interface{}, resourceUsage *cstructs.TaskResourceUsage) error {
	ru, err := e.Impl.Stats()
	if ru != nil {
		*resourceUsage = *ru
	}
	return err
}

func (e *ExecutorRPCServer) Signal(args os.Signal, resp *interface{}) error {
	return e.Impl.Signal(args)
}

func (e *ExecutorRPCServer) Exec(args ExecCmdArgs, result *ExecCmdReturn) error {
	out, code, err := e.Impl.Exec(args.Deadline, args.Name, args.Args)
	ret := &ExecCmdReturn{
		Output: out,
		Code:   code,
	}
	*result = *ret
	return err
}

type ExecutorPlugin struct {
	logger *log.Logger
	Impl   *ExecutorRPCServer
}

func (p *ExecutorPlugin) Server(*plugin.MuxBroker) (interface{}, error) {
	if p.Impl == nil {
		p.Impl = &ExecutorRPCServer{Impl: executor.NewExecutor(p.logger), logger: p.logger}
	}
	return p.Impl, nil
}

func (p *ExecutorPlugin) Client(b *plugin.MuxBroker, c *rpc.Client) (interface{}, error) {
	return &ExecutorRPC{client: c, logger: p.logger}, nil
}
