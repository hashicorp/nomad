package taskrunner

import (
	"context"
	"fmt"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocrunner/getter"
	"github.com/hashicorp/nomad/client/allocrunnerv2/interfaces"
	ti "github.com/hashicorp/nomad/client/allocrunnerv2/taskrunner/interfaces"
	"github.com/hashicorp/nomad/nomad/structs"
)

// artifactHook downloads artifacts for a task.
type artifactHook struct {
	eventEmitter ti.EventEmitter
	logger       log.Logger
}

func newArtifactHook(e ti.EventEmitter, logger log.Logger) *artifactHook {
	h := &artifactHook{
		eventEmitter: e,
	}
	h.logger = logger.Named(h.Name())
	return h
}

func (*artifactHook) Name() string {
	return "artifacts"
}

func (h *artifactHook) Prestart(ctx context.Context, req *interfaces.TaskPrestartRequest, resp *interfaces.TaskPrestartResponse) error {
	h.eventEmitter.SetState(structs.TaskStatePending, structs.NewTaskEvent(structs.TaskDownloadingArtifacts))

	for _, artifact := range req.Task.Artifacts {
		//XXX add ctx to GetArtifact to allow cancelling long downloads
		if err := getter.GetArtifact(req.TaskEnv, artifact, req.TaskDir); err != nil {
			wrapped := fmt.Errorf("failed to download artifact %q: %v", artifact.GetterSource, err)
			h.logger.Debug(wrapped.Error())
			h.eventEmitter.SetState(structs.TaskStatePending,
				structs.NewTaskEvent(structs.TaskArtifactDownloadFailed).SetDownloadError(wrapped))
			return wrapped
		}
	}

	resp.Done = true
	return nil
}
