// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	log "github.com/hashicorp/go-hclog"

	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/users"
	"github.com/hashicorp/nomad/nomad/structs"
)

// identityHook sets the task runner's Nomad workload identity token
// based on the signed identity stored on the Allocation

const (
	// wiTokenFile is the name of the file holding the Nomad token inside the
	// task's secret directory
	wiTokenFile = "nomad_token"
)

// IdentitySigner is the interface needed to retrieve signed identities for
// workload identities. At runtime it is implemented by *widmgr.WIDMgr.
type IdentitySigner interface {
	SignIdentities(minIndex uint64, req []*structs.WorkloadIdentityRequest) ([]*structs.SignedWorkloadIdentity, error)
}

type identityHook struct {
	tr       *TaskRunner
	tokenDir string
	logger   log.Logger

	stopCtx context.Context
	stop    context.CancelFunc
}

func newIdentityHook(tr *TaskRunner, logger log.Logger) *identityHook {
	// Create a context for the renew loop. This context will be canceled when
	// the task is stopped or agent is shutting down, unlike Prestart's ctx which
	// is not intended for use after Prestart is returns.
	stopCtx, stop := context.WithCancel(context.Background())

	h := &identityHook{
		tr:       tr,
		tokenDir: tr.taskDir.SecretsDir,
		stopCtx:  stopCtx,
		stop:     stop,
	}
	h.logger = logger.Named(h.Name())
	return h
}

func (*identityHook) Name() string {
	return "identity"
}

func (h *identityHook) Prestart(ctx context.Context, req *interfaces.TaskPrestartRequest, resp *interfaces.TaskPrestartResponse) error {

	// Handle default workload identity
	if err := h.setDefaultToken(); err != nil {
		return err
	}

	signedWIDs, err := h.getIdentities(req.Alloc, req.Task)
	if err != nil {
		return fmt.Errorf("error fetching alternate identities: %w", err)
	}

	for _, widspec := range req.Task.Identities {
		signedWID := signedWIDs[widspec.Name]
		if signedWID == nil {
			// The only way to hit this should be a bug as it indicates the server
			// did not sign an identity for a task on this alloc.
			return fmt.Errorf("missing workload identity %q", widspec.Name)
		}

		if err := h.setAltToken(widspec, signedWID.JWT); err != nil {
			return err
		}
	}

	// Start token renewal loop
	go h.renew(req.Alloc.CreateIndex, signedWIDs)

	return nil
}

func (h *identityHook) Stop(context.Context, *interfaces.TaskStopRequest, *interfaces.TaskStopResponse) error {
	h.stop()
	return nil
}

// setDefaultToken adds the Nomad token to the task's environment and writes it to a
// file if requested by the jobsepc.
func (h *identityHook) setDefaultToken() error {
	token := h.tr.alloc.SignedIdentities[h.tr.taskName]
	if token == "" {
		return nil
	}

	// Handle internal use and env var
	h.tr.setNomadToken(token)

	task := h.tr.Task()

	// Handle file writing
	if id := task.Identity; id != nil && id.File {
		// Write token as owner readable only
		tokenPath := filepath.Join(h.tokenDir, wiTokenFile)
		if err := users.WriteFileFor(tokenPath, []byte(token), task.User); err != nil {
			return fmt.Errorf("failed to write nomad token: %w", err)
		}
	}

	return nil
}

// setAltToken takes an alternate workload identity and sets the env var and/or
// writes the token file as specified by the jobspec.
func (h *identityHook) setAltToken(widspec *structs.WorkloadIdentity, rawJWT string) error {
	if widspec.Env {
		h.tr.envBuilder.SetWorkloadToken(widspec.Name, rawJWT)
	}

	if widspec.File {
		tokenPath := filepath.Join(h.tokenDir, fmt.Sprintf("nomad_%s.jwt", widspec.Name))
		if err := users.WriteFileFor(tokenPath, []byte(rawJWT), h.tr.Task().User); err != nil {
			return fmt.Errorf("failed to write token for identity %q: %w", widspec.Name, err)
		}
	}

	return nil
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
	signedWIDs, err := h.tr.widmgr.SignIdentities(alloc.CreateIndex, req)
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

func (h *identityHook) renew(createIndex uint64, signedWIDs map[string]*structs.SignedWorkloadIdentity) {
	wids := h.tr.Task().Identities
	if len(wids) == 0 {
		h.logger.Trace("no workload identities to renew")
		return
	}

	var reqs []*structs.WorkloadIdentityRequest
	var minExp time.Time
	widMap := make(map[string]*structs.WorkloadIdentity, len(wids)) // Identity.Name -> Identity

	for _, wid := range wids {
		if wid.TTL == 0 {
			// No ttl, so no need to renew it
			continue
		}

		widMap[wid.Name] = wid

		reqs = append(reqs, &structs.WorkloadIdentityRequest{
			AllocID:      h.tr.allocID,
			TaskName:     h.tr.taskName,
			IdentityName: wid.Name,
		})

		sid, ok := signedWIDs[wid.Name]
		if !ok {
			// Missing a signature, treat this case as already expired so we get a
			// token ASAP
			minExp = time.Time{}
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

	const minWait = 10 * time.Second
	wait := helper.ExpiryToRenewTime(minExp, time.Now, minWait)

	timer, timerStop := helper.NewStoppedTimer()
	defer timerStop()

	var retry uint64

	for err := h.stopCtx.Err(); err == nil; {
		timer.Reset(wait)
		select {
		case <-timer.C:
			h.logger.Trace("getting new signed identities", "num", len(reqs))
		case <-h.stopCtx.Done():
			return
		}

		// Renew all tokens together since its cheap
		tokens, err := h.tr.widmgr.SignIdentities(createIndex, reqs)
		if err != nil {
			retry++
			wait = helper.Backoff(minWait, time.Hour, retry) + helper.RandomStagger(minWait)
			h.logger.Error("error renewing workload identities", "error", err, "next", wait)
			continue
		}

		if len(tokens) == 0 {
			retry++
			wait = helper.Backoff(minWait, time.Hour, retry) + helper.RandomStagger(minWait)
			h.logger.Error("error renewing workload identities", "error", "no tokens", "next", wait)
			continue
		}

		// Reset next expiration time
		minExp = time.Time{}

		for _, token := range tokens {
			widspec, ok := widMap[token.IdentityName]
			if !ok {
				// Bug: Every requested workload identity should either have a signed
				// identity.
				h.logger.Warn("bug: unexpected workload identity received", "identity", token.IdentityName)
				continue
			}

			if err := h.setAltToken(widspec, token.JWT); err != nil {
				// Set minExp using retry's backoff logic
				minExp = time.Now().Add(helper.Backoff(minWait, time.Hour, retry+1) + helper.RandomStagger(minWait))
				h.logger.Error("error setting new workload identity", "error", err, "identity", token.IdentityName)
				continue
			}

			// Set next expiration time
			if minExp.IsZero() {
				minExp = token.Expiration
			} else if token.Expiration.Before(minExp) {
				minExp = token.Expiration
			}
		}

		// Success! Set next renewal and reset retries
		wait = helper.ExpiryToRenewTime(minExp, time.Now, minWait)
		retry = 0

		h.logger.Debug("waitng to renew workloading identities", "next", wait)
	}
}
