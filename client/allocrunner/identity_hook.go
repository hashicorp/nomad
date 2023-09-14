// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocrunner

import (
	"context"
	"fmt"
	"time"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
)

// IdentitySigner is the interface needed to retrieve signed identities for
// workload identities. At runtime it is implemented by *widmgr.WIDMgr.
type IdentitySigner interface {
	SignIdentities(minIndex uint64, req []*structs.WorkloadIdentityRequest) ([]*structs.SignedWorkloadIdentity, error)
}

type identityHook struct {
	ar            *allocRunner
	hookResources *cstructs.AllocHookResources
	widmgr        IdentitySigner
	logger        log.Logger

	// minWait is the minimum amount of time to wait before renewing. Settable to
	// ease testing.
	minWait time.Duration

	stopCtx context.Context
	stop    context.CancelFunc
}

func newIdentityHook(ar *allocRunner, hookResources *cstructs.AllocHookResources, logger log.Logger) *identityHook {
	// Create a context for the renew loop. This context will be canceled when
	// the allocation is stopped or agent is shutting down
	stopCtx, stop := context.WithCancel(context.Background())

	h := &identityHook{
		ar:            ar,
		hookResources: hookResources,
		minWait:       10 * time.Second,
		stopCtx:       stopCtx,
		stop:          stop,
	}
	h.logger = logger.Named(h.Name())
	return h
}

func (*identityHook) Name() string {
	return "identity"
}

func (h *identityHook) Prerun() error {
	signedWIDs := map[string]*structs.SignedWorkloadIdentity{}

	// we need to know the amount of tasks to create a buffered
	// SignedTaskIdentities channel
	numberOfTasks := len(h.ar.tasks)

	for _, t := range h.ar.tasks {
		task := t.Task()
		if task == nil {
			// hitting this means a bug, but better safe than sorry
			continue
		}

		var err error
		signedWIDs, err = h.getIdentities(h.ar.Alloc(), task)
		if err != nil {
			return fmt.Errorf("error fetching alternate identities: %w", err)
		}

		// publish task identities inside hookResources, so that taskrunner
		// hooks can also use them.
		h.hookResources.SignedTaskIdentities = make(chan map[string]*structs.SignedWorkloadIdentity, numberOfTasks)
		h.hookResources.SignedTaskIdentities <- signedWIDs
	}

	// Start token renewal loop
	go h.renew(h.ar.alloc.CreateIndex, signedWIDs)

	return nil
}

// Stop implements interfaces.TaskStopHook
func (h *identityHook) Stop(context.Context, *interfaces.TaskStopRequest, *interfaces.TaskStopResponse) error {
	h.stop()
	return nil
}

// Shutdown implements interfaces.ShutdownHook
func (h *identityHook) Shutdown() {
	h.stop()
}

// getIdentities calls Alloc.SignIdentities to get all of the identities for
// this workload signed. If there are no identities to be signed then (nil,
// nil) is returned.
func (h *identityHook) getIdentities(alloc *structs.Allocation, task *structs.Task) (map[string]*structs.SignedWorkloadIdentity, error) {

	if len(task.Identities) == 0 {
		return nil, nil
	}

	req := make([]*structs.WorkloadIdentityRequest, len(task.Identities))
	for i, widspec := range task.Identities {
		req[i] = &structs.WorkloadIdentityRequest{
			AllocID:      alloc.ID,
			TaskName:     task.Name,
			IdentityName: widspec.Name,
		}
	}

	// Get signed workload identities
	signedWIDs, err := h.ar.widmgr.SignIdentities(alloc.CreateIndex, req)
	if err != nil {
		return nil, err
	}

	// Index initial workload identities by name
	widMap := make(map[string]*structs.SignedWorkloadIdentity, len(signedWIDs))
	for _, wid := range signedWIDs {
		widMap[wid.IdentityName] = wid
	}

	return widMap, nil
}

// renew fetches new signed workload identity tokens before the existing tokens
// expire.
func (h *identityHook) renew(createIndex uint64, signedWIDs map[string]*structs.SignedWorkloadIdentity) {
	for _, t := range h.ar.tasks {
		alloc := h.ar.Alloc()
		task := t.Task()

		wids := task.Identities
		if len(wids) == 0 {
			h.logger.Trace("no workload identities to renew")
			return
		}

		var reqs []*structs.WorkloadIdentityRequest
		renewNow := false
		minExp := time.Now().Add(30 * time.Hour)                        // set high default expiration
		widMap := make(map[string]*structs.WorkloadIdentity, len(wids)) // Identity.Name -> Identity

		for _, wid := range wids {
			if wid.TTL == 0 {
				// No ttl, so no need to renew it
				continue
			}

			widMap[wid.Name] = wid

			reqs = append(reqs, &structs.WorkloadIdentityRequest{
				AllocID:      alloc.ID,
				TaskName:     task.Name,
				IdentityName: wid.Name,
			})

			sid, ok := signedWIDs[wid.Name]
			if !ok {
				// Missing a signature, treat this case as already expired so we get a
				// token ASAP
				h.logger.Trace("missing token for identity", "identity", wid.Name)
				renewNow = true
				continue
			}

			if sid.Expiration.Before(minExp) {
				minExp = sid.Expiration
			}
		}

		if len(reqs) == 0 {
			h.logger.Trace("no workload identities expire")
			return
		}

		var wait time.Duration
		if !renewNow {
			wait = helper.ExpiryToRenewTime(minExp, time.Now, h.minWait)
		}

		timer, timerStop := helper.NewStoppedTimer()
		defer timerStop()

		var retry uint64

		for {
			// we need to handle stopCtx.Err() and manually stop the subscribers
			if err := h.stopCtx.Err(); err != nil {
				h.hookResources.StopChan <- struct{}{}
				return
			}

			h.logger.Debug("waiting to renew identities", "num", len(reqs), "wait", wait)
			timer.Reset(wait)
			select {
			case <-timer.C:
				h.logger.Trace("getting new signed identities", "num", len(reqs))
			case <-h.stopCtx.Done():
				h.hookResources.StopChan <- struct{}{}
				return
			}

			// Renew all tokens together since its cheap
			tokens, err := h.widmgr.SignIdentities(createIndex, reqs)
			if err != nil {
				retry++
				wait = helper.Backoff(h.minWait, time.Hour, retry) + helper.RandomStagger(h.minWait)
				h.logger.Error("error renewing workload identities", "error", err, "next", wait)
				continue
			}

			if len(tokens) == 0 {
				retry++
				wait = helper.Backoff(h.minWait, time.Hour, retry) + helper.RandomStagger(h.minWait)
				h.logger.Error("error renewing workload identities", "error", "no tokens", "next", wait)
				continue
			}

			// Reset next expiration time
			minExp = time.Time{}

			for _, token := range tokens {
				// Set next expiration time
				if minExp.IsZero() {
					minExp = token.Expiration
				} else if token.Expiration.Before(minExp) {
					minExp = token.Expiration
				}
			}

			// Publish updates for taskrunner consumers
			h.hookResources.SignedTaskIdentities <- signedWIDs

			// Success! Set next renewal and reset retries
			wait = helper.ExpiryToRenewTime(minExp, time.Now, h.minWait)
			retry = 0
		}
	}
}
