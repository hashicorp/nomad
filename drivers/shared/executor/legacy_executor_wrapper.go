package executor

import (
	"os"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/drivers/shared/executor/legacy"
	"github.com/hashicorp/nomad/plugins/drivers"
	"golang.org/x/net/context"
)

type legacyExecutorWrapper struct {
	client *legacy.ExecutorRPC
	logger hclog.Logger
}

func (l *legacyExecutorWrapper) Launch(launchCmd *ExecCommand) (*ProcessState, error) {
	panic("not implemented")
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
	if err := l.client.ShutDown(); err != nil {
		return err
	}

	if err := l.client.Exit(); err != nil {
		return err
	}
	return nil
}

func (l *legacyExecutorWrapper) UpdateResources(*drivers.Resources) error {
	panic("not implemented")
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
