// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/helper/users"
)

// apiHook exposes the Task API. The Task API allows task's to access the Nomad
// HTTP API without having to discover and connect to an agent's address.
// Instead a unix socket is provided in a standard location. To prevent access
// by untrusted workloads the Task API always requires authentication even when
// ACLs are disabled.
//
// The Task API hook largely soft-fails as there are a number of ways creating
// the unix socket could fail (the most common one being path length
// restrictions), and it is assumed most tasks won't require access to the Task
// API anyway. Tasks that do require access are expected to crash and get
// rescheduled should they land on a client who Task API hook soft-fails.
type apiHook struct {
	shutdownCtx context.Context
	srv         config.APIListenerRegistrar
	logger      hclog.Logger

	// Lock listener as it is updated from multiple hooks.
	lock sync.Mutex

	// Listener is the unix domain socket of the task api for this taks.
	ln net.Listener
}

func newAPIHook(shutdownCtx context.Context, srv config.APIListenerRegistrar, logger hclog.Logger) *apiHook {
	h := &apiHook{
		shutdownCtx: shutdownCtx,
		srv:         srv,
	}
	h.logger = logger.Named(h.Name())
	return h
}

func (*apiHook) Name() string {
	return "api"
}

func (h *apiHook) Prestart(_ context.Context, req *interfaces.TaskPrestartRequest, resp *interfaces.TaskPrestartResponse) error {
	h.lock.Lock()
	defer h.lock.Unlock()

	if h.ln != nil {
		// Listener already set. Task is probably restarting.
		return nil
	}

	udsPath := apiSocketPath(req.TaskDir)
	udsln, err := users.SocketFileFor(h.logger, udsPath, req.Task.User)
	if err != nil {
		// Soft-fail and let the task fail if it requires the task api.
		h.logger.Warn("error creating task api socket", "path", udsPath, "error", err)
		return nil
	}

	go func() {
		// Cannot use Prestart's context as it is closed after all prestart hooks
		// have been closed, but we do want to try to cleanup on shutdown.
		if err := h.srv.Serve(h.shutdownCtx, udsln); err != nil {
			if errors.Is(err, http.ErrServerClosed) {
				return
			}
			if errors.Is(err, net.ErrClosed) {
				return
			}
			h.logger.Error("error serving task api", "error", err)
		}
	}()

	h.ln = udsln
	return nil
}

func (h *apiHook) Stop(ctx context.Context, req *interfaces.TaskStopRequest, resp *interfaces.TaskStopResponse) error {
	h.lock.Lock()
	defer h.lock.Unlock()

	if h.ln != nil {
		if err := h.ln.Close(); err != nil {
			if !errors.Is(err, net.ErrClosed) {
				h.logger.Debug("error closing task listener: %v", err)
			}
		}
		h.ln = nil
	}

	// Best-effort at cleaining things up. Alloc dir cleanup will remove it if
	// this fails for any reason.
	_ = os.RemoveAll(apiSocketPath(req.TaskDir))

	return nil
}

// apiSocketPath returns the path to the Task API socket.
//
// The path needs to be as short as possible because of the low limits on the
// sun_path char array imposed by the syscall used to create unix sockets.
//
// See https://github.com/hashicorp/nomad/pull/13971 for an example of the
// sadness this causes.
func apiSocketPath(taskDir *allocdir.TaskDir) string {
	return filepath.Join(taskDir.SecretsDir, "api.sock")
}
