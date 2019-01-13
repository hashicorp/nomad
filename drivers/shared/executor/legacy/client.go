package legacy

import (
	"encoding/gob"
	"net/rpc"
	"os"
	"syscall"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
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

type ProcessState struct {
	Pid      int
	ExitCode int
	Signal   int
	Time     time.Time
}

type ExecutorVersion struct {
	Version string
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

func (e *ExecutorRPC) Wait() (*ProcessState, error) {
	var ps ProcessState
	err := e.client.Call("Plugin.Wait", new(interface{}), &ps)
	return &ps, err
}

func (e *ExecutorRPC) ShutDown() error {
	return e.client.Call("Plugin.ShutDown", new(interface{}), new(interface{}))
}

func (e *ExecutorRPC) Exit() error {
	return e.client.Call("Plugin.Exit", new(interface{}), new(interface{}))
}

func (e *ExecutorRPC) Version() (*ExecutorVersion, error) {
	var version ExecutorVersion
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

type ExecutorPlugin struct {
	logger hclog.Logger
}

func NewExecutorPlugin(logger hclog.Logger) plugin.Plugin {
	return &ExecutorPlugin{logger: logger}
}

func (p *ExecutorPlugin) Client(b *plugin.MuxBroker, c *rpc.Client) (interface{}, error) {
	return &ExecutorRPC{client: c, logger: p.logger}, nil
}

func (p *ExecutorPlugin) Server(*plugin.MuxBroker) (interface{}, error) {
	panic("client only supported")
}
