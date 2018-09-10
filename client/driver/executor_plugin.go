package driver

import (
	"encoding/gob"
	"net/rpc"
	"os"
	"syscall"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/client/driver/executor"
	cstructs "github.com/hashicorp/nomad/client/structs"
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
	logger hclog.Logger
}

// LaunchCmdArgs wraps a user command and the args for the purposes of RPC
type LaunchArgs struct {
	Cmd *executor.ExecCommand
}

type ExecArgs struct {
	Deadline time.Time
	Name     string
	Args     []string
}

type ExecReturn struct {
	Output []byte
	Code   int
}

func (e *ExecutorRPC) Launch(cmd *executor.ExecCommand) (*executor.ProcessState, error) {
	var ps *executor.ProcessState
	err := e.client.Call("Plugin.Launch", LaunchArgs{Cmd: cmd}, &ps)
	return ps, err
}

func (e *ExecutorRPC) Wait() (*executor.ProcessState, error) {
	var ps executor.ProcessState
	err := e.client.Call("Plugin.Wait", new(interface{}), &ps)
	return &ps, err
}

func (e *ExecutorRPC) Kill() error {
	return e.client.Call("Plugin.Kill", new(interface{}), new(interface{}))
}

func (e *ExecutorRPC) Destroy() error {
	return e.client.Call("Plugin.Destroy", new(interface{}), new(interface{}))
}

func (e *ExecutorRPC) UpdateResources(resources *executor.Resources) error {
	return e.client.Call("Plugin.UpdateResources", resources, new(interface{}))
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
	req := ExecArgs{
		Deadline: deadline,
		Name:     name,
		Args:     args,
	}
	var resp *ExecReturn
	err := e.client.Call("Plugin.Exec", req, &resp)
	if resp == nil {
		return nil, 0, err
	}
	return resp.Output, resp.Code, err
}

type ExecutorRPCServer struct {
	Impl   executor.Executor
	logger hclog.Logger
}

func (e *ExecutorRPCServer) Launch(args LaunchArgs, ps *executor.ProcessState) error {
	state, err := e.Impl.Launch(args.Cmd)
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

func (e *ExecutorRPCServer) Kill(args interface{}, resp *interface{}) error {
	return e.Impl.Kill()
}

func (e *ExecutorRPCServer) Destroy(args interface{}, resp *interface{}) error {
	return e.Impl.Destroy()
}

func (e *ExecutorRPCServer) UpdateResources(args *executor.Resources, resp *interface{}) error {
	return e.Impl.UpdateResources(args)
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
	logger hclog.Logger
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
