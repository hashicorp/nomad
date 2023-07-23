package taskrunner

import (
	"github.com/shoenig/netlog"

	"context"

	"github.com/hashicorp/go-hclog"
	ifs "github.com/hashicorp/nomad/client/allocrunner/interfaces"
	cifs "github.com/hashicorp/nomad/client/interfaces"
	"github.com/hashicorp/nomad/client/lib/proclib"
)

const (
	wranglerHookName = "wrangler"
)

type wranglerHook struct {
	wranglers cifs.ProcessWranglers
	task      proclib.Task
	log       hclog.Logger
}

func newWranglerHook(wranglers cifs.ProcessWranglers, task, allocID string, log hclog.Logger) *wranglerHook {

	netlog.Cyan("whook", "task", task, "allocID", allocID)

	return &wranglerHook{
		wranglers: wranglers,
		log:       log.Named(wranglerHookName),
		task: proclib.Task{
			AllocID: allocID,
			Task:    task,
		},
	}
}

func (*wranglerHook) Name() string {
	return wranglerHookName
}

func (wh *wranglerHook) Prestart(_ context.Context, request *ifs.TaskPrestartRequest, _ *ifs.TaskPrestartResponse) error {
	wh.log.Info("setting up process management", "task", wh.task)
	return wh.wranglers.Setup(wh.task)
}

func (wh *wranglerHook) Stop(_ context.Context, request *ifs.TaskStopRequest, _ *ifs.TaskStopResponse) error {
	wh.log.Info("stopping process mangagement", "task", wh.task)
	return wh.wranglers.Destroy(wh.task)
}
