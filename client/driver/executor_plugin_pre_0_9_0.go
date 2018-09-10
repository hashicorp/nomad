package driver

import (
	"encoding/gob"
	"log"
	"net/rpc"
	"os"
	"syscall"
	"time"

	"github.com/hashicorp/go-plugin"
	executorv0 "github.com/hashicorp/nomad/client/driver/executorv0"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/structs"
)

// Registering these types since we have to serialize and de-serialize the Task
// structs over the wire between drivers and the executorv0.
func init() {
	gob.Register([]interface{}{})
	gob.Register(map[string]interface{}{})
	gob.Register([]map[string]string{})
	gob.Register([]map[string]int{})
	gob.Register(syscall.Signal(0x1))
}

type ExecutorRPCPre0_9_0 struct {
	client *rpc.Client
	logger *log.Logger
}

// LaunchCmdArgs wraps a user command and the args for the purposes of RPC
type LaunchCmdArgs struct {
	Cmd *executorv0.ExecCommand
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

func (e *ExecutorRPCPre0_9_0) LaunchCmd(cmd *executorv0.ExecCommand) (*executorv0.ProcessState, error) {
	var ps *executorv0.ProcessState
	err := e.client.Call("Plugin.LaunchCmd", LaunchCmdArgs{Cmd: cmd}, &ps)
	return ps, err
}

func (e *ExecutorRPCPre0_9_0) LaunchSyslogServer() (*executorv0.SyslogServerState, error) {
	var ss *executorv0.SyslogServerState
	err := e.client.Call("Plugin.LaunchSyslogServer", new(interface{}), &ss)
	return ss, err
}

func (e *ExecutorRPCPre0_9_0) Wait() (*executorv0.ProcessState, error) {
	var ps executorv0.ProcessState
	err := e.client.Call("Plugin.Wait", new(interface{}), &ps)
	return &ps, err
}

func (e *ExecutorRPCPre0_9_0) ShutDown() error {
	return e.client.Call("Plugin.ShutDown", new(interface{}), new(interface{}))
}

func (e *ExecutorRPCPre0_9_0) Exit() error {
	return e.client.Call("Plugin.Exit", new(interface{}), new(interface{}))
}

func (e *ExecutorRPCPre0_9_0) SetContext(ctx *executorv0.ExecutorContext) error {
	return e.client.Call("Plugin.SetContext", ctx, new(interface{}))
}

func (e *ExecutorRPCPre0_9_0) UpdateLogConfig(logConfig *executorv0.LogConfig) error {
	return e.client.Call("Plugin.UpdateLogConfig", logConfig, new(interface{}))
}

func (e *ExecutorRPCPre0_9_0) UpdateTask(task *structs.Task) error {
	return e.client.Call("Plugin.UpdateTask", task, new(interface{}))
}

func (e *ExecutorRPCPre0_9_0) DeregisterServices() error {
	return e.client.Call("Plugin.DeregisterServices", new(interface{}), new(interface{}))
}

func (e *ExecutorRPCPre0_9_0) Version() (*executorv0.ExecutorVersion, error) {
	var version executorv0.ExecutorVersion
	err := e.client.Call("Plugin.Version", new(interface{}), &version)
	return &version, err
}

func (e *ExecutorRPCPre0_9_0) Stats() (*cstructs.TaskResourceUsage, error) {
	var resourceUsage cstructs.TaskResourceUsage
	err := e.client.Call("Plugin.Stats", new(interface{}), &resourceUsage)
	return &resourceUsage, err
}

func (e *ExecutorRPCPre0_9_0) Signal(s os.Signal) error {
	return e.client.Call("Plugin.Signal", &s, new(interface{}))
}

func (e *ExecutorRPCPre0_9_0) Exec(deadline time.Time, name string, args []string) ([]byte, int, error) {
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

type ExecutorRPCServerPre0_9_0 struct {
	Impl   executorv0.Executor
	logger *log.Logger
}

func (e *ExecutorRPCServerPre0_9_0) LaunchCmd(args LaunchCmdArgs, ps *executorv0.ProcessState) error {
	state, err := e.Impl.LaunchCmd(args.Cmd)
	if state != nil {
		*ps = *state
	}
	return err
}

func (e *ExecutorRPCServerPre0_9_0) LaunchSyslogServer(args interface{}, ss *executorv0.SyslogServerState) error {
	state, err := e.Impl.LaunchSyslogServer()
	if state != nil {
		*ss = *state
	}
	return err
}

func (e *ExecutorRPCServerPre0_9_0) Wait(args interface{}, ps *executorv0.ProcessState) error {
	state, err := e.Impl.Wait()
	if state != nil {
		*ps = *state
	}
	return err
}

func (e *ExecutorRPCServerPre0_9_0) ShutDown(args interface{}, resp *interface{}) error {
	return e.Impl.ShutDown()
}

func (e *ExecutorRPCServerPre0_9_0) Exit(args interface{}, resp *interface{}) error {
	return e.Impl.Exit()
}

func (e *ExecutorRPCServerPre0_9_0) SetContext(args *executorv0.ExecutorContext, resp *interface{}) error {
	return e.Impl.SetContext(args)
}

func (e *ExecutorRPCServerPre0_9_0) UpdateLogConfig(args *executorv0.LogConfig, resp *interface{}) error {
	return e.Impl.UpdateLogConfig(args)
}

func (e *ExecutorRPCServerPre0_9_0) UpdateTask(args *structs.Task, resp *interface{}) error {
	return e.Impl.UpdateTask(args)
}

func (e *ExecutorRPCServerPre0_9_0) DeregisterServices(args interface{}, resp *interface{}) error {
	// In 0.6 this is a noop. Goes away in 0.7.
	return nil
}

func (e *ExecutorRPCServerPre0_9_0) Version(args interface{}, version *executorv0.ExecutorVersion) error {
	ver, err := e.Impl.Version()
	if ver != nil {
		*version = *ver
	}
	return err
}

func (e *ExecutorRPCServerPre0_9_0) Stats(args interface{}, resourceUsage *cstructs.TaskResourceUsage) error {
	ru, err := e.Impl.Stats()
	if ru != nil {
		*resourceUsage = *ru
	}
	return err
}

func (e *ExecutorRPCServerPre0_9_0) Signal(args os.Signal, resp *interface{}) error {
	return e.Impl.Signal(args)
}

func (e *ExecutorRPCServerPre0_9_0) Exec(args ExecCmdArgs, result *ExecCmdReturn) error {
	out, code, err := e.Impl.Exec(args.Deadline, args.Name, args.Args)
	ret := &ExecCmdReturn{
		Output: out,
		Code:   code,
	}
	*result = *ret
	return err
}

type ExecutorPluginPre0_9_0 struct {
	logger *log.Logger
	Impl   *ExecutorRPCServerPre0_9_0
}

func (p *ExecutorPluginPre0_9_0) Server(*plugin.MuxBroker) (interface{}, error) {
	if p.Impl == nil {
		p.Impl = &ExecutorRPCServerPre0_9_0{Impl: executorv0.NewExecutor(p.logger), logger: p.logger}
	}
	return p.Impl, nil
}

func (p *ExecutorPluginPre0_9_0) Client(b *plugin.MuxBroker, c *rpc.Client) (interface{}, error) {
	return &ExecutorRPCPre0_9_0{client: c, logger: p.logger}, nil
}
