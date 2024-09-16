// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocrunner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
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

const (
	consulGRPCSockHookName = "consul_grpc_socket"

	// socketProxyStopWaitTime is the amount of time to wait for a socket proxy
	// to stop before assuming something went awry and return a timeout error.
	socketProxyStopWaitTime = 3 * time.Second

	// consulGRPCFallbackPort is the last resort fallback port to use in
	// combination with the Consul HTTP config address when creating the
	// socket.
	consulGRPCFallbackPort = "8502"
)

var (
	errSocketProxyTimeout = errors.New("timed out waiting for socket proxy to exit")
)

// consulGRPCSocketHook creates Unix sockets to allow communication from inside a
// netns to Consul gRPC endpoint.
//
// Noop for allocations without a group Connect block using bridge networking.
type consulGRPCSocketHook struct {
	logger hclog.Logger

	// mu synchronizes proxy and alloc which may be mutated and read concurrently
	// via Prerun, Update, Postrun.
	mu      sync.Mutex
	alloc   *structs.Allocation
	proxies map[string]*grpcSocketProxy
}

func newConsulGRPCSocketHook(
	logger hclog.Logger,
	alloc *structs.Allocation,
	allocDir allocdir.Interface,
	configs map[string]*config.ConsulConfig,
	nodeAttrs map[string]string,
) *consulGRPCSocketHook {

	// Get the deduplicated set of Consul clusters that are needed by this
	// alloc. For Nomad CE, this will always be just the default cluster.
	clusterNames := set.New[string](1)
	tg := alloc.Job.LookupTaskGroup(alloc.TaskGroup)
	for _, s := range tg.Services {
		clusterNames.Insert(s.GetConsulClusterName(tg))
	}
	proxies := map[string]*grpcSocketProxy{}

	for clusterName := range clusterNames.Items() {
		// Attempt to find the gRPC port via the node attributes, otherwise use
		// the default fallback.
		attrName := "consul.grpc"
		if clusterName != structs.ConsulDefaultCluster {
			attrName = "consul." + clusterName + ".grpc"
		}
		consulGRPCPort, ok := nodeAttrs[attrName]
		if !ok {
			consulGRPCPort = consulGRPCFallbackPort
		}

		proxies[clusterName] = newGRPCSocketProxy(
			logger,
			allocDir,
			configs[clusterName],
			consulGRPCPort,
		)
	}

	return &consulGRPCSocketHook{
		alloc:   alloc,
		proxies: proxies,
		logger:  logger.Named(consulGRPCSockHookName),
	}
}

func (*consulGRPCSocketHook) Name() string {
	return consulGRPCSockHookName
}

// shouldRun returns true if the Unix socket should be created and proxied.
// Requires the mutex to be held.
func (h *consulGRPCSocketHook) shouldRun() bool {
	tg := h.alloc.Job.LookupTaskGroup(h.alloc.TaskGroup)

	// we must be in bridge networking and at least one connect sidecar task
	if !tgFirstNetworkIsBridge(tg) {
		return false
	}

	for _, s := range tg.Services {
		if s.Connect.HasSidecar() || s.Connect.IsGateway() {
			return true
		}
	}

	return false
}

func (h *consulGRPCSocketHook) Prerun() error {
	h.mu.Lock()
	defer h.mu.Unlock()

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

// Update creates a gRPC socket file and proxy if there are any Connect
// services.
func (h *consulGRPCSocketHook) Update(req *interfaces.RunnerUpdateRequest) error {
	h.mu.Lock()
	defer h.mu.Unlock()

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

func (h *consulGRPCSocketHook) Postrun() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	for _, proxy := range h.proxies {
		if err := proxy.stop(); err != nil {
			// Only log failures to stop proxies. Worst case scenario is a small
			// goroutine leak.
			h.logger.Warn("error stopping Consul proxy", "error", err)
		}
	}

	return nil
}

type grpcSocketProxy struct {
	logger   hclog.Logger
	allocDir allocdir.Interface
	config   *config.ConsulConfig

	// consulGRPCFallbackPort is the port to use if the operator did not
	// specify a gRPC config address.
	consulGRPCFallbackPort string

	ctx     context.Context
	cancel  func()
	doneCh  chan struct{}
	runOnce bool
}

func newGRPCSocketProxy(
	logger hclog.Logger,
	allocDir allocdir.Interface,
	config *config.ConsulConfig,
	consulGRPCFallbackPort string,
) *grpcSocketProxy {

	ctx, cancel := context.WithCancel(context.Background())
	return &grpcSocketProxy{
		allocDir:               allocDir,
		config:                 config,
		consulGRPCFallbackPort: consulGRPCFallbackPort,
		ctx:                    ctx,
		cancel:                 cancel,
		doneCh:                 make(chan struct{}),
		logger:                 logger,
	}
}

// run socket proxy if allocation requires it, it isn't already running, and it
// hasn't been told to stop.
//
// NOT safe for concurrent use.
func (p *grpcSocketProxy) run(alloc *structs.Allocation) error {
	// Only run once.
	if p.runOnce {
		return nil
	}

	// Only run once. Never restart.
	select {
	case <-p.doneCh:
		p.logger.Trace("socket proxy already shutdown; exiting")
		return nil
	case <-p.ctx.Done():
		p.logger.Trace("socket proxy already done; exiting")
		return nil
	default:
	}

	// make sure either grpc or http consul address has been configured
	if p.config.GRPCAddr == "" && p.config.Addr == "" {
		return fmt.Errorf("consul address for cluster %q must be set on nomad client",
			p.config.Name)
	}

	destAddr := p.config.GRPCAddr
	if destAddr == "" {
		// No GRPCAddr defined. Use Addr but replace port with the gRPC
		// default of 8502.
		host, _, err := net.SplitHostPort(p.config.Addr)
		if err != nil {
			return fmt.Errorf("error parsing Consul address %q: %v", p.config.Addr, err)
		}
		destAddr = net.JoinHostPort(host, p.consulGRPCFallbackPort)
	}

	socketFile := allocdir.AllocGRPCSocket
	if p.config.Name != structs.ConsulDefaultCluster && p.config.Name != "" {
		socketFile = filepath.Join(allocdir.SharedAllocName, allocdir.TmpDirName,
			"consul_"+p.config.Name+"_grpc.sock")
	}
	hostGRPCSocketPath := filepath.Join(p.allocDir.AllocDirPath(), socketFile)

	// if the socket already exists we'll try to remove it, but if not then any
	// other errors will bubble up to the caller here or when we try to listen
	_, err := os.Stat(hostGRPCSocketPath)
	if err == nil {
		err := os.Remove(hostGRPCSocketPath)
		if err != nil {
			return fmt.Errorf(
				"unable to remove existing unix socket for Consul gRPC endpoint: %v", err)
		}
	}

	listener, err := net.Listen("unix", hostGRPCSocketPath)
	if err != nil {
		return fmt.Errorf("unable to create unix socket for Consul gRPC endpoint: %v", err)
	}

	// The gRPC socket should be usable by all users in case a task is
	// running as an unprivileged user.  Unix does not allow setting domain
	// socket permissions when creating the file, so we must manually call
	// chmod afterwards.
	// https://github.com/golang/go/issues/11822
	if err := os.Chmod(hostGRPCSocketPath, os.ModePerm); err != nil {
		return fmt.Errorf("unable to set permissions on unix socket for Consul gRPC endpoint: %v", err)
	}

	go func() {
		proxy(p.ctx, p.logger, destAddr, listener)
		p.cancel()
		close(p.doneCh)
	}()

	p.runOnce = true
	return nil
}

// stop the proxy and blocks until the proxy has stopped. Returns an error if
// the proxy does not exit in a timely fashion.
func (p *grpcSocketProxy) stop() error {
	p.cancel()

	// If proxy was never run, don't wait for anything to shutdown.
	if !p.runOnce {
		return nil
	}

	select {
	case <-p.doneCh:
		return nil
	case <-time.After(socketProxyStopWaitTime):
		return errSocketProxyTimeout
	}
}

// Proxy between a listener and destination.
func proxy(ctx context.Context, logger hclog.Logger, destAddr string, l net.Listener) {
	// Wait for all connections to be done before exiting to prevent
	// goroutine leaks.
	wg := sync.WaitGroup{}
	ctx, cancel := context.WithCancel(ctx)
	defer func() {
		// Must cancel context and close listener before waiting
		cancel()
		_ = l.Close()
		wg.Wait()
	}()

	// Close Accept() when context is cancelled
	go func() {
		<-ctx.Done()
		_ = l.Close()
	}()

	for ctx.Err() == nil {
		conn, err := l.Accept()
		if err != nil {
			if ctx.Err() != nil {
				// Accept errors during shutdown are to be expected
				return
			}
			logger.Error("error in socket proxy; shutting down proxy", "error", err, "dest", destAddr)
			return
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			proxyConn(ctx, logger, destAddr, conn)
		}()
	}
}

// proxyConn proxies between an existing net.Conn and a destination address. If
// the destAddr starts with "unix://" it is treated as a path to a unix socket.
// Otherwise it is treated as a host for a TCP connection.
//
// When the context is cancelled proxyConn blocks until all goroutines shutdown
// to prevent leaks.
func proxyConn(ctx context.Context, logger hclog.Logger, destAddr string, conn net.Conn) {
	// Close the connection when we're done with it.
	defer conn.Close()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Detect unix sockets
	network := "tcp"
	const unixPrefix = "unix://"
	if strings.HasPrefix(destAddr, unixPrefix) {
		network = "unix"
		destAddr = destAddr[len(unixPrefix):]
	}

	dialer := &net.Dialer{}
	dest, err := dialer.DialContext(ctx, network, destAddr)
	if err == context.Canceled || err == context.DeadlineExceeded {
		logger.Trace("proxy exiting gracefully", "error", err, "dest", destAddr,
			"src_local", conn.LocalAddr(), "src_remote", conn.RemoteAddr())
		return
	}
	if err != nil {
		logger.Error("error connecting to grpc", "error", err, "dest", destAddr)
		return
	}

	// Wait for goroutines to exit before exiting to prevent leaking.
	wg := sync.WaitGroup{}
	defer wg.Wait()

	// socket -> consul
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cancel()
		n, err := io.Copy(dest, conn)
		if ctx.Err() == nil && err != nil {
			// expect disconnects when proxying http
			logger.Trace("error message received proxying to Consul",
				"msg", err, "dest", destAddr, "src_local", conn.LocalAddr(),
				"src_remote", conn.RemoteAddr(), "bytes", n)
			return
		}
		logger.Trace("proxy to Consul complete",
			"src_local", conn.LocalAddr(), "src_remote", conn.RemoteAddr(),
			"bytes", n,
		)
	}()

	// consul -> socket
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cancel()
		n, err := io.Copy(conn, dest)
		if ctx.Err() == nil && err != nil {
			logger.Trace("error message received proxying from Consul",
				"msg", err, "dest", destAddr, "src_local", conn.LocalAddr(),
				"src_remote", conn.RemoteAddr(), "bytes", n)
			return
		}
		logger.Trace("proxy from Consul complete",
			"src_local", conn.LocalAddr(), "src_remote", conn.RemoteAddr(),
			"bytes", n,
		)
	}()

	// When cancelled close connections to break out of copies goroutines.
	<-ctx.Done()
	_ = conn.Close()
	_ = dest.Close()
}
