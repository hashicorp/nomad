package executor

import (
	"encoding/gob"
	"fmt"
	"net/rpc"
	"os"
	"syscall"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	plugin "github.com/hashicorp/go-plugin"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
	"golang.org/x/net/context"
)

const (
	// pre09DockerSignal is used in executor.Shutdown to know if it should
	// call the ShutDown RPC on the pre09 executor
	pre09DockerSignal = "docker"
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

type legacyExecutorWrapper struct {
	client *pre09ExecutorRPC
	logger hclog.Logger
}

func (l *legacyExecutorWrapper) Launch(launchCmd *ExecCommand) (*ProcessState, error) {
	return nil, fmt.Errorf("operation not supported for legacy exec wrapper")
}

func (l *legacyExecutorWrapper) Wait(ctx context.Context) (*ProcessState, error) {
	ps, err := l.client.Wait()
	if err != nil {
		return nil, err
	}

	return &ProcessState{
		Pid:      ps.Pid,
		ExitCode: ps.ExitCode,
		Signal:   ps.Signal,
		Time:     ps.Time,
	}, nil
}

func (l *legacyExecutorWrapper) Shutdown(signal string, gracePeriod time.Duration) error {
	// The legacy docker driver only used the executor to start a syslog server
	// for logging. Thus calling ShutDown for docker will always return an error
	// because it never started a process through the executor. If signal is set
	// to 'docker' then we'll skip the ShutDown RPC and just call Exit.
	//
	// This is painful to look at but will only be around a few releases
	if signal != pre09DockerSignal {
		if err := l.client.ShutDown(); err != nil {
			return err
		}
	}

	if err := l.client.Exit(); err != nil {
		return err
	}
	return nil
}

func (l *legacyExecutorWrapper) UpdateResources(*drivers.Resources) error {
	return fmt.Errorf("operation not supported for legacy exec wrapper")
}

func (l *legacyExecutorWrapper) Version() (*ExecutorVersion, error) {
	v, err := l.client.Version()
	if err != nil {
		return nil, err
	}

	return &ExecutorVersion{
		Version: v.Version,
	}, nil
}

func (l *legacyExecutorWrapper) Stats(ctx context.Context, interval time.Duration) (<-chan *cstructs.TaskResourceUsage, error) {
	ch := make(chan *cstructs.TaskResourceUsage, 1)
	stats, err := l.client.Stats()
	if err != nil {
		close(ch)
		return nil, err
	}
	select {
	case ch <- stats:
	default:
	}
	go l.handleStats(ctx, interval, ch)
	return ch, nil
}

func (l *legacyExecutorWrapper) handleStats(ctx context.Context, interval time.Duration, ch chan *cstructs.TaskResourceUsage) {
	defer close(ch)
	ticker := time.NewTicker(interval)
	for range ticker.C {
		stats, err := l.client.Stats()
		if err != nil {
			if err == rpc.ErrShutdown {
				return
			}
			l.logger.Warn("stats collection from legacy executor failed, waiting for next interval", "error", err)
			continue
		}
		if stats != nil {
			select {
			case ch <- stats:
			default:
			}
		}

	}
}

func (l *legacyExecutorWrapper) Signal(s os.Signal) error {
	return l.client.Signal(s)
}

func (l *legacyExecutorWrapper) Exec(deadline time.Time, cmd string, args []string) ([]byte, int, error) {
	return l.client.Exec(deadline, cmd, args)
}

type pre09ExecutorRPC struct {
	client *rpc.Client
	logger hclog.Logger
}

type pre09ExecCmdArgs struct {
	Deadline time.Time
	Name     string
	Args     []string
}

type pre09ExecCmdReturn struct {
	Output []byte
	Code   int
}

func (e *pre09ExecutorRPC) Wait() (*ProcessState, error) {
	var ps ProcessState
	err := e.client.Call("Plugin.Wait", new(interface{}), &ps)
	return &ps, err
}

func (e *pre09ExecutorRPC) ShutDown() error {
	return e.client.Call("Plugin.ShutDown", new(interface{}), new(interface{}))
}

func (e *pre09ExecutorRPC) Exit() error {
	return e.client.Call("Plugin.Exit", new(interface{}), new(interface{}))
}

func (e *pre09ExecutorRPC) Version() (*ExecutorVersion, error) {
	var version ExecutorVersion
	err := e.client.Call("Plugin.Version", new(interface{}), &version)
	return &version, err
}

func (e *pre09ExecutorRPC) Stats() (*cstructs.TaskResourceUsage, error) {
	var resourceUsage cstructs.TaskResourceUsage
	err := e.client.Call("Plugin.Stats", new(interface{}), &resourceUsage)
	return &resourceUsage, err
}

func (e *pre09ExecutorRPC) Signal(s os.Signal) error {
	return e.client.Call("Plugin.Signal", &s, new(interface{}))
}

func (e *pre09ExecutorRPC) Exec(deadline time.Time, name string, args []string) ([]byte, int, error) {
	req := pre09ExecCmdArgs{
		Deadline: deadline,
		Name:     name,
		Args:     args,
	}
	var resp *pre09ExecCmdReturn
	err := e.client.Call("Plugin.Exec", req, &resp)
	if resp == nil {
		return nil, 0, err
	}
	return resp.Output, resp.Code, err
}

type pre09ExecutorPlugin struct {
	logger hclog.Logger
}

func newPre09ExecutorPlugin(logger hclog.Logger) plugin.Plugin {
	return &pre09ExecutorPlugin{logger: logger}
}

func (p *pre09ExecutorPlugin) Client(b *plugin.MuxBroker, c *rpc.Client) (interface{}, error) {
	return &legacyExecutorWrapper{
		client: &pre09ExecutorRPC{client: c, logger: p.logger},
		logger: p.logger,
	}, nil
}

func (p *pre09ExecutorPlugin) Server(*plugin.MuxBroker) (interface{}, error) {
	return nil, fmt.Errorf("client only supported")
}
