package taskrunner

import (
	"context"
	"fmt"
	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/allocrunner/taskrunner/getter"
	ti "github.com/hashicorp/nomad/client/allocrunner/taskrunner/interfaces"
	"github.com/hashicorp/nomad/nomad/structs"
	"sync"
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

func (h *artifactHook) createWorkers(req *interfaces.TaskPrestartRequest, resp *interfaces.TaskPrestartResponse, noOfWorkers int, jobsChannel chan *structs.TaskArtifact, errorChannel chan error) {
	var wg sync.WaitGroup
	for i := 0; i < noOfWorkers; i++ {
		wg.Add(1)
		go h.doWork(req, resp, jobsChannel, errorChannel, &wg)
	}
	wg.Wait()
	close(errorChannel)
}

func (h *artifactHook) doWork(req *interfaces.TaskPrestartRequest, resp *interfaces.TaskPrestartResponse, jobs chan *structs.TaskArtifact, errorChannel chan error, wg *sync.WaitGroup) {
	for artifact := range jobs {
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

			errorChannel <- herr
			continue
		}

		// Mark artifact as downloaded to avoid re-downloading due to
		// retries caused by subsequent artifacts failing. Any
		// non-empty value works.
		resp.State[aid] = "1"
	}
	wg.Done()
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

	// maxConcurrency denotes the number of workers that will download artifacts in parallel
	maxConcurrency := 3

	// jobsChannel is a buffered channel which will have all the artifacts that needs to be processed
	jobsChannel := make(chan *structs.TaskArtifact, maxConcurrency)
	// Push all artifact requests to job channel
	go func() {
		for _, artifact := range req.Task.Artifacts {
			jobsChannel <- artifact
		}
		close(jobsChannel)
	}()

	errorChannel := make(chan error, maxConcurrency)
	// create workers and process artifacts
	h.createWorkers(req, resp, maxConcurrency, jobsChannel, errorChannel)

	// Iterate over the errorChannel and if there is an error, return it
	for err := range errorChannel {
		if err != nil {
			return err
		}
	}

	resp.Done = true
	return nil
}
