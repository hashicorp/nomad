// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-msgpack/v2/codec"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/config"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/users"
	"github.com/hashicorp/nomad/nomad/structs"
)

type proxyHook struct {
	region      string
	namespace   string
	shutdownCtx context.Context
	rpcClient   config.RPCer
	logger      hclog.Logger

	// Lock listener as it is updated from multiple hooks.
	lock sync.Mutex

	// Listeners are the unix domain sockets for upstream services.
	listeners map[string]net.Listener
}

func newProxyHook(shutdownCtx context.Context, rpcC config.RPCer, logger hclog.Logger, alloc *structs.Allocation) *proxyHook {
	h := &proxyHook{
		listeners:   map[string]net.Listener{},
		shutdownCtx: shutdownCtx,
		rpcClient:   rpcC,
		region:      alloc.Job.Region,
		namespace:   alloc.Namespace,
	}
	h.logger = logger.Named(h.Name())
	return h
}

func (*proxyHook) Name() string {
	return "proxy"
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
			for h.shutdownCtx.Err() == nil {
				uc, err := udsln.Accept()
				if err != nil {
					// TODO(schmichael) idk
					h.logger.Warn("error accepting connection for service proxy", "service", name, "error", err)
					return
				}

				go h.serve(name, uc)

			}
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

func (h *proxyHook) serve(name string, localConn net.Conn) {
	handler, err := h.rpcClient.RemoteStreamingRpcHandler("ServiceRegistration.Proxy")
	if err != nil {
		h.logger.Error("unable to initiate service proxy rpc", "error", err)
		return
	}

	localPipe, remotePipe := net.Pipe()
	decoder := codec.NewDecoder(localPipe, structs.MsgpackHandle)
	encoder := codec.NewEncoder(localPipe, structs.MsgpackHandle)

	// Start goroutine to read from remote peer
	go func() {
		defer localPipe.Close()
		for {
			var wrapper cstructs.StreamErrWrapper

			if err := decoder.Decode(&wrapper); err != nil {
				h.logger.Warn("error decoding payload from service", "service", name, "error", err)
				return
			}

			if wrapper.Error != nil {
				h.logger.Debug("received error from remote peer", "service", name, "error", err)
				return
			}

			if n, err := localConn.Write(wrapper.Payload); n != len(wrapper.Payload) || err != nil {
				encoder.Encode(&cstructs.StreamErrWrapper{
					Error: cstructs.NewRpcError(
						fmt.Errorf("error writing to local task from service %q", name),
						pointer.Of(int64(http.StatusServiceUnavailable)),
					),
				})
				return
			}
		}
	}()

	// Start goroutine to write to remote peer
	go func() {
		defer localPipe.Close()

		args := &cstructs.ServiceProxyRequest{
			ServiceName: name,
			QueryOptions: structs.QueryOptions{
				Region:    h.region,
				Namespace: h.namespace,
			},
		}

		if err := encoder.Encode(args); err != nil {
			h.logger.Error("error encoding rpc to service", "service", name, "error", err)
			return
		}

		// Now proxy traffic from local task as wrapped stream payloads
		for {
			buf := make([]byte, 64*1024)
			n, err := localConn.Read(buf)
			if n > 0 {
				err := encoder.Encode(&cstructs.StreamErrWrapper{
					Payload: buf[:n],
				})
				if err != nil {
					h.logger.Warn("error encoding payload from task for service", "service", name, "error", err)
					return
				}
			}

			if err != nil {
				if err == io.EOF {
					return
				}

				encoder.Encode(&cstructs.StreamErrWrapper{
					Error: cstructs.NewRpcError(
						fmt.Errorf("error reading from local task for service %q", name),
						pointer.Of(int64(http.StatusServiceUnavailable)),
					),
				})
				return
			}
		}
	}()

	handler(remotePipe)
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
