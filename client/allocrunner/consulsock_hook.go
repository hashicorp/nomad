package allocrunner

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/structs/config"
)

// consulSockHook creates Unix sockets to allow communication from inside a
// netns to Consul.
//
// Noop for allocations without a group Connect stanza.
type consulSockHook struct {
	alloc *structs.Allocation

	proxy *sockProxy

	// mu synchronizes group & cancel as they may be mutated and accessed
	// concurrently via Prerun, Update, Postrun.
	mu sync.Mutex

	logger hclog.Logger
}

func newConsulSockHook(logger hclog.Logger, alloc *structs.Allocation, allocDir *allocdir.AllocDir, config *config.ConsulConfig) *consulSockHook {
	h := &consulSockHook{
		alloc: alloc,
		proxy: newSockProxy(logger, allocDir, config),
	}
	h.logger = logger.Named(h.Name())
	return h
}

func (*consulSockHook) Name() string {
	return "consul_socket"
}

// shouldRun returns true if the Unix socket should be created and proxied.
// Requires the mutex to be held.
func (h *consulSockHook) shouldRun() bool {
	tg := h.alloc.Job.LookupTaskGroup(h.alloc.TaskGroup)
	for _, s := range tg.Services {
		if s.Connect != nil {
			return true
		}
	}

	return false
}

func (h *consulSockHook) Prerun() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if !h.shouldRun() {
		return nil
	}

	return h.proxy.run(h.alloc)
}

// Update creates a gRPC socket file and proxy if there are any Connect
// services.
func (h *consulSockHook) Update(req *interfaces.RunnerUpdateRequest) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.alloc = req.Alloc

	if !h.shouldRun() {
		return nil
	}

	return h.proxy.run(h.alloc)
}

func (h *consulSockHook) Postrun() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if err := h.proxy.stop(); err != nil {
		// Only log failures to stop proxies. Worst case scenario is a
		// small goroutine leak.
		h.logger.Debug("error stopping Consul proxy", "error", err)
	}
	return nil
}

type sockProxy struct {
	allocDir *allocdir.AllocDir
	config   *config.ConsulConfig

	ctx     context.Context
	cancel  func()
	doneCh  chan struct{}
	runOnce bool

	logger hclog.Logger
}

func newSockProxy(logger hclog.Logger, allocDir *allocdir.AllocDir, config *config.ConsulConfig) *sockProxy {
	ctx, cancel := context.WithCancel(context.Background())
	return &sockProxy{
		allocDir: allocDir,
		config:   config,
		ctx:      ctx,
		cancel:   cancel,
		doneCh:   make(chan struct{}),
		logger:   logger,
	}
}

// run socket proxy if allocation requires it, it isn't already running, and it
// hasn't been told to stop.
//
// NOT safe for concurrent use.
func (s *sockProxy) run(alloc *structs.Allocation) error {
	// Only run once.
	if s.runOnce {
		return nil
	}

	// Only run once. Never restart.
	select {
	case <-s.doneCh:
		s.logger.Trace("socket proxy already shutdown; exiting")
		return nil
	case <-s.ctx.Done():
		s.logger.Trace("socket proxy already done; exiting")
		return nil
	default:
	}

	destAddr := s.config.GRPCAddr
	if destAddr == "" {
		// No GRPCAddr defined. Use Addr but replace port with the gRPC
		// default of 8502.
		host, _, err := net.SplitHostPort(s.config.Addr)
		if err != nil {
			return fmt.Errorf("error parsing Consul address %q: %v",
				s.config.Addr, err)
		}

		destAddr = net.JoinHostPort(host, "8502")
	}

	hostGRPCSockPath := filepath.Join(s.allocDir.AllocDir, allocdir.AllocGRPCSocket)

	// if the socket already exists we'll try to remove it, but if not then any
	// other errors will bubble up to the caller here or when we try to listen
	_, err := os.Stat(hostGRPCSockPath)
	if err == nil {
		err := os.Remove(hostGRPCSockPath)
		if err != nil {
			return fmt.Errorf(
				"unable to remove existing unix socket for Consul gRPC endpoint: %v", err)
		}
	}

	listener, err := net.Listen("unix", hostGRPCSockPath)
	if err != nil {
		return fmt.Errorf("unable to create unix socket for Consul gRPC endpoint: %v", err)
	}

	// The gRPC socket should be usable by all users in case a task is
	// running as an unprivileged user.  Unix does not allow setting domain
	// socket permissions when creating the file, so we must manually call
	// chmod afterwards.
	// https://github.com/golang/go/issues/11822
	if err := os.Chmod(hostGRPCSockPath, os.ModePerm); err != nil {
		return fmt.Errorf("unable to set permissions on unix socket for Consul gRPC endpoint: %v", err)
	}

	go func() {
		proxy(s.ctx, s.logger, destAddr, listener)
		s.cancel()
		close(s.doneCh)
	}()

	s.runOnce = true
	return nil
}

// stop the proxy and blocks until the proxy has stopped. Returns an error if
// the proxy does not exit in a timely fashion.
func (s *sockProxy) stop() error {
	s.cancel()

	// If proxy was never run, don't wait for anything to shutdown.
	if !s.runOnce {
		return nil
	}

	select {
	case <-s.doneCh:
		return nil
	case <-time.After(3 * time.Second):
		return fmt.Errorf("timed out waiting for proxy to exit")
	}
}

// Proxy between a listener and dest
func proxy(ctx context.Context, logger hclog.Logger, dest string, l net.Listener) {
	// Wait for all connections to be done before exiting to prevent
	// goroutine leaks.
	wg := sync.WaitGroup{}
	ctx, cancel := context.WithCancel(ctx)
	defer func() {
		// Must cancel context and close listener before waiting
		cancel()
		l.Close()
		wg.Wait()
	}()

	// Close Accept() when context is cancelled
	go func() {
		<-ctx.Done()
		l.Close()
	}()

	for ctx.Err() == nil {
		conn, err := l.Accept()
		if err != nil {
			if ctx.Err() != nil {
				// Accept errors during shutdown are to be expected
				return
			}
			logger.Error("error in grpc proxy; shutting down proxy", "error", err, "dest", dest)
			return
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			proxyConn(ctx, logger, dest, conn)
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

	// socket -> gRPC
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cancel()
		n, err := io.Copy(dest, conn)
		if ctx.Err() == nil && err != nil {
			logger.Warn("error proxying to Consul", "error", err, "dest", destAddr,
				"src_local", conn.LocalAddr(), "src_remote", conn.RemoteAddr(),
				"bytes", n,
			)
			return
		}
		logger.Trace("proxy to Consul complete",
			"src_local", conn.LocalAddr(), "src_remote", conn.RemoteAddr(),
			"bytes", n,
		)
	}()

	// gRPC -> socket
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cancel()
		n, err := io.Copy(conn, dest)
		if ctx.Err() == nil && err != nil {
			logger.Warn("error proxying from Consul", "error", err, "dest", destAddr,
				"src_local", conn.LocalAddr(), "src_remote", conn.RemoteAddr(),
				"bytes", n,
			)
			return
		}
		logger.Trace("proxy from Consul complete",
			"src_local", conn.LocalAddr(), "src_remote", conn.RemoteAddr(),
			"bytes", n,
		)
	}()

	// When cancelled close connections to break out of copies goroutines.
	<-ctx.Done()
	conn.Close()
	dest.Close()
}
