// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocrunner

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/go-set/v3"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/structs/config"
)

func tgFirstNetworkIsBridge(tg *structs.TaskGroup) bool {
	if len(tg.Networks) < 1 || tg.Networks[0].Mode != "bridge" {
		return false
	}
	return true
}

const (
	consulHTTPSocketHookName = "consul_http_socket"
)

type consulHTTPSockHook struct {
	logger hclog.Logger

	// lock synchronizes proxy and alloc which may be mutated and read concurrently
	// via Prerun, Update, and Postrun.
	lock    sync.Mutex
	alloc   *structs.Allocation
	proxies map[string]*httpSocketProxy
}

func newConsulHTTPSocketHook(
	logger hclog.Logger,
	alloc *structs.Allocation,
	allocDir allocdir.Interface,
	configs map[string]*config.ConsulConfig,
) *consulHTTPSockHook {

	// Get the deduplicated set of Consul clusters that are needed by this
	// alloc. For Nomad CE, this will always be just the default cluster.
	clusterNames := set.New[string](1)
	tg := alloc.Job.LookupTaskGroup(alloc.TaskGroup)
	for _, s := range tg.Services {
		clusterNames.Insert(s.GetConsulClusterName(tg))
	}
	proxies := map[string]*httpSocketProxy{}

	for clusterName := range clusterNames.Items() {
		proxies[clusterName] = newHTTPSocketProxy(
			logger,
			allocDir,
			configs[clusterName],
		)
	}

	return &consulHTTPSockHook{
		alloc:   alloc,
		proxies: proxies,
		logger:  logger.Named(consulHTTPSocketHookName),
	}
}

func (*consulHTTPSockHook) Name() string {
	return consulHTTPSocketHookName
}

// shouldRun returns true if the alloc contains at least one connect native
// task and has a network configured in bridge mode
//
// todo(shoenig): what about CNI networks?
func (h *consulHTTPSockHook) shouldRun() bool {
	tg := h.alloc.Job.LookupTaskGroup(h.alloc.TaskGroup)

	// we must be in bridge networking and at least one connect native task
	if !tgFirstNetworkIsBridge(tg) {
		return false
	}

	for _, service := range tg.Services {
		if service.Connect.IsNative() {
			return true
		}
	}
	return false
}

func (h *consulHTTPSockHook) Prerun() error {
	h.lock.Lock()
	defer h.lock.Unlock()

	if !h.shouldRun() {
		return nil
	}

	var mErr *multierror.Error
	for _, proxy := range h.proxies {
		if err := proxy.run(h.alloc); err != nil {
			mErr = multierror.Append(mErr, err)
		}
	}
	return mErr.ErrorOrNil()
}

func (h *consulHTTPSockHook) Update(req *interfaces.RunnerUpdateRequest) error {
	h.lock.Lock()
	defer h.lock.Unlock()

	h.alloc = req.Alloc

	if !h.shouldRun() {
		return nil
	}
	if len(h.proxies) == 0 {
		return fmt.Errorf("cannot update alloc to Connect in-place")
	}

	var mErr *multierror.Error
	for _, proxy := range h.proxies {
		if err := proxy.run(h.alloc); err != nil {
			mErr = multierror.Append(mErr, err)
		}
	}
	return mErr.ErrorOrNil()
}

func (h *consulHTTPSockHook) Postrun() error {
	h.lock.Lock()
	defer h.lock.Unlock()

	for _, proxy := range h.proxies {
		if err := proxy.stop(); err != nil {
			// Only log failures to stop proxies. Worst case scenario is a small
			// goroutine leak.
			h.logger.Warn("error stopping Consul HTTP proxy", "error", err)
		}
	}

	return nil
}

type httpSocketProxy struct {
	logger   hclog.Logger
	allocDir allocdir.Interface
	config   *config.ConsulConfig

	ctx     context.Context
	cancel  func()
	doneCh  chan struct{}
	runOnce bool
}

func newHTTPSocketProxy(
	logger hclog.Logger,
	allocDir allocdir.Interface,
	config *config.ConsulConfig,
) *httpSocketProxy {
	ctx, cancel := context.WithCancel(context.Background())
	return &httpSocketProxy{
		logger:   logger,
		allocDir: allocDir,
		config:   config,
		ctx:      ctx,
		cancel:   cancel,
		doneCh:   make(chan struct{}),
	}
}

// run the httpSocketProxy for the given allocation.
//
// Assumes locking done by the calling alloc runner.
func (p *httpSocketProxy) run(alloc *structs.Allocation) error {
	// Only run once.
	if p.runOnce {
		return nil
	}

	// Never restart.
	select {
	case <-p.doneCh:
		p.logger.Trace("consul http socket proxy already shutdown; exiting")
		return nil
	case <-p.ctx.Done():
		p.logger.Trace("consul http socket proxy already done; exiting")
		return nil
	default:
	}

	// consul http dest addr
	destAddr := p.config.Addr
	if destAddr == "" {
		return errors.New("consul address must be set on nomad client")
	}

	socketFile := allocdir.AllocHTTPSocket
	if p.config.Name != structs.ConsulDefaultCluster && p.config.Name != "" {
		socketFile = filepath.Join(allocdir.SharedAllocName, allocdir.TmpDirName,
			"consul_"+p.config.Name+"_http.sock")
	}
	hostHTTPSockPath := filepath.Join(p.allocDir.AllocDirPath(), socketFile)
	if err := maybeRemoveOldSocket(hostHTTPSockPath); err != nil {
		return err
	}

	listener, err := net.Listen("unix", hostHTTPSockPath)
	if err != nil {
		return fmt.Errorf("unable to create unix socket for Consul HTTP endpoint: %w", err)
	}

	// The Consul HTTP socket should be usable by all users in case a task is
	// running as a non-privileged user. Unix does not allow setting domain
	// socket permissions when creating the file, so we must manually call
	// chmod afterwards.
	if err := os.Chmod(hostHTTPSockPath, os.ModePerm); err != nil {
		return fmt.Errorf("unable to set permissions on unix socket: %w", err)
	}

	go func() {
		proxy(p.ctx, p.logger, destAddr, listener)
		p.cancel()
		close(p.doneCh)
	}()

	p.runOnce = true
	return nil
}

func (p *httpSocketProxy) stop() error {
	p.cancel()

	// if proxy was never run, no need to wait before shutdown
	if !p.runOnce {
		return nil
	}

	select {
	case <-p.doneCh:
	case <-time.After(socketProxyStopWaitTime):
		return errSocketProxyTimeout
	}

	return nil
}

func maybeRemoveOldSocket(socketPath string) error {
	_, err := os.Stat(socketPath)
	if err == nil {
		if err = os.Remove(socketPath); err != nil {
			return fmt.Errorf("unable to remove existing unix socket: %w", err)
		}
	}
	return nil
}
