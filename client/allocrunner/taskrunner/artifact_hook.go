// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"fmt"
	"sync"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	ti "github.com/hashicorp/nomad/client/allocrunner/taskrunner/interfaces"
	ci "github.com/hashicorp/nomad/client/interfaces"
	"github.com/hashicorp/nomad/nomad/structs"
)

// artifactHook downloads artifacts for a task.
type artifactHook struct {
	eventEmitter ti.EventEmitter
	logger       log.Logger
	getter       ci.ArtifactGetter
}

func newArtifactHook(e ti.EventEmitter, getter ci.ArtifactGetter, logger log.Logger) *artifactHook {
	h := &artifactHook{
		eventEmitter: e,
		getter:       getter,
	}
	h.logger = logger.Named(h.Name())
	return h
}

func (h *artifactHook) doWork(req *interfaces.TaskPrestartRequest, resp *interfaces.TaskPrestartResponse, jobs chan *structs.TaskArtifact, errorChannel chan error, wg *sync.WaitGroup, responseStateMutex *sync.Mutex) {
	defer wg.Done()
	for artifact := range jobs {
		aid := artifact.Hash()
		if req.PreviousState[aid] != "" {
			h.logger.Trace("skipping already downloaded artifact", "artifact", artifact.GetterSource)
			responseStateMutex.Lock()
			resp.State[aid] = req.PreviousState[aid]
			responseStateMutex.Unlock()
			continue
		}

		h.logger.Debug("downloading artifact", "artifact", artifact.GetterSource, "aid", aid)

		if err := h.getter.Get(req.TaskEnv, artifact); err != nil {
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
		responseStateMutex.Lock()
		resp.State[aid] = "1"
		responseStateMutex.Unlock()
	}
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

	// responseStateMutex is a lock used to guard against concurrent writes to the above resp.State map
	responseStateMutex := &sync.Mutex{}

	h.eventEmitter.EmitEvent(structs.NewTaskEvent(structs.TaskDownloadingArtifacts))

	// maxConcurrency denotes the number of workers that will download artifacts in parallel
	maxConcurrency := 3

	// jobsChannel is a buffered channel which will have all the artifacts that needs to be processed
	jobsChannel := make(chan *structs.TaskArtifact, maxConcurrency)

	// errorChannel is also a buffered channel that will be used to signal errors
	errorChannel := make(chan error, maxConcurrency)

	// create workers and process artifacts
	go func() {
		defer close(errorChannel)
		var wg sync.WaitGroup
		for i := 0; i < maxConcurrency; i++ {
			wg.Add(1)
			go h.doWork(req, resp, jobsChannel, errorChannel, &wg, responseStateMutex)
		}
		wg.Wait()
	}()

	// Push all artifact requests to job channel
	go func() {
		defer close(jobsChannel)
		for _, artifact := range req.Task.Artifacts {
			jobsChannel <- artifact
		}
	}()

	// Iterate over the errorChannel and if there is an error, store it to a variable for future return
	var err error
	for e := range errorChannel {
		err = e
	}

	// once error channel is closed, we can check and return the error
	if err != nil {
		return err
	}

	resp.Done = true
	return nil
}
