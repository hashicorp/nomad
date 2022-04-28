package taskrunner

import (
	"context"
	"fmt"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/allocrunner/taskrunner/getter"
	ti "github.com/hashicorp/nomad/client/allocrunner/taskrunner/interfaces"
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
	// Copied in client/state when upgrading from <0.9 schemas, so if you
	// change it here you also must change it there.
	return "artifacts"
}

func (h *artifactHook) Prestart(ctx context.Context, req *interfaces.TaskPrestartRequest, resp *interfaces.TaskPrestartResponse) error {
	if len(req.Task.Artifacts) == 0 {
		resp.Done = true
		return nil
	}

	// Initialize hook state to store download progress
	resp.State = make(map[string]string, len(req.Task.Artifacts))

	h.eventEmitter.EmitEvent(structs.NewTaskEvent(structs.TaskDownloadingArtifacts))

	for _, artifact := range req.Task.Artifacts {
		aid := artifact.Hash()
		if req.PreviousState[aid] != "" {
			h.logger.Trace("skipping already downloaded artifact", "artifact", artifact.GetterSource)
			resp.State[aid] = req.PreviousState[aid]
			continue
		}

		h.logger.Debug("downloading artifact", "artifact", artifact.GetterSource)
		//XXX add ctx to GetArtifact to allow cancelling long downloads
		if err := getter.GetArtifact(req.TaskEnv, artifact); err != nil {

			wrapped := structs.NewRecoverableError(
				fmt.Errorf("failed to download artifact %q: %v", artifact.GetterSource, err),
				true,
			)
			herr := NewHookError(wrapped, structs.NewTaskEvent(structs.TaskArtifactDownloadFailed).SetDownloadError(wrapped))

			return herr
		}

		// Mark artifact as downloaded to avoid re-downloading due to
		// retries caused by subsequent artifacts failing. Any
		// non-empty value works.
		resp.State[aid] = "1"
	}

	resp.Done = true
	return nil
}
