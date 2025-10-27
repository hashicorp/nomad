// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/helper/users"
)

// proxyHook exposes the Task API. The Task API allows task's to access the Nomad
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
type proxyHook struct {
	shutdownCtx context.Context
	srv         config.APIListenerRegistrar
	logger      hclog.Logger

	// Lock listener as it is updated from multiple hooks.
	lock sync.Mutex

	// Listeners are the unix domain sockets for upstream services.
	listeners map[string]net.Listener
}

func newProxyHook(shutdownCtx context.Context, srv config.APIListenerRegistrar, logger hclog.Logger) *proxyHook {
	h := &proxyHook{
		listeners:   map[string]net.Listener{},
		shutdownCtx: shutdownCtx,
		srv:         srv,
	}
	h.logger = logger.Named(h.Name())
	return h
}

func (*proxyHook) Name() string {
	return "api"
}

func (h *proxyHook) Prestart(_ context.Context, req *interfaces.TaskPrestartRequest, resp *interfaces.TaskPrestartResponse) error {
	h.lock.Lock()
	defer h.lock.Unlock()

	for _, serviceName := range req.Task.Upstreams {
		if ln := h.listeners[serviceName]; ln != nil {
			// Listener already set. Task is probably restarting.
			continue
		}

		udsPath := proxySocketPath(req.TaskDir, serviceName)
		udsln, err := users.SocketFileFor(h.logger, udsPath, req.Task.User)
		if err != nil {
			// TODO(schmichael) TaskAPI soft fails here because few workloads
			// actually require the TaskAPI. who knows what the right call here is so
			// uh... die loudly to make debugging easier?
			return fmt.Errorf("error creating service proxy socket %s: %w", udsPath, err)
		}

		go func(name string) {
			for h.shutdownCtx.Err() != nil {
				uc, err := udsln.AcceptUnix()
				if err != nil {
					// TODO(schmichael) idk
					h.logger.Warn("error accepting connection for service proxy", "service", name, "error", err)
					return
				}

			}
			panic("TODO")
		}(serviceName)

		h.listeners[serviceName] = udsln
	}

	return nil
}

func (h *proxyHook) Stop(ctx context.Context, req *interfaces.TaskStopRequest, resp *interfaces.TaskStopResponse) error {
	h.lock.Lock()
	defer h.lock.Unlock()

	for k, ln := range h.listeners {
		if ln == nil {
			continue
		}
		if err := ln.Close(); err != nil {
			if !errors.Is(err, net.ErrClosed) {
				h.logger.Debug("error closing service proxy listener", "error", err)
			}
		}
		h.listeners[k] = nil

		// Best-effort at cleaining things up. Alloc dir cleanup will remove it if
		// this fails for any reason.
		_ = os.RemoveAll(proxySocketPath(req.TaskDir, k))
	}

	return nil
}

// proxySocketPath returns the path to the Task API socket.
//
// The path needs to be as short as possible because of the low limits on the
// sun_path char array imposed by the syscall used to create unix sockets.
//
// See https://github.com/hashicorp/nomad/pull/13971 for an example of the
// sadness this causes.
func proxySocketPath(taskDir *allocdir.TaskDir, name string) string {
	return filepath.Join(taskDir.SecretsDir, name+".sock")
}
