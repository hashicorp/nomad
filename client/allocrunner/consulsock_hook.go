package allocrunner

import (
	"context"
	"io"
	"net"
	"net/url"
	"path/filepath"
	"strings"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/nomad/structs/config"
)

// consulSockHook creates unix sockets to allow communication from inside a
// netns to Consul.
type consulSockHook struct {
	allocDir *allocdir.AllocDir
	config   *config.ConsulConfig

	// cancel shuts down any running proxies. May be nil.
	cancel func()

	logger hclog.Logger
}

func newConsulSockHook(logger hclog.Logger, allocDir *allocdir.AllocDir, config *config.ConsulConfig) *consulSockHook {
	h := &consulSockHook{
		allocDir: allocDir,
		config:   config,
	}
	h.logger = logger.Named(h.Name())
	return h
}

func (consulSockHook) Name() string {
	return "consul_socket"
}

func (h *consulSockHook) Prerun() error {
	destAddr := h.config.GRPCAddr
	if destAddr == "" {
		// No GRPCAddr defined. Use Addr but replace port with the gRPC
		// default of 8502.
		destURL, err := url.Parse(h.config.Addr)
		if err != nil {
			h.logger.Warn("unable to create unix sockets for Consul; can not determine Consul gRPC endpoint",
				"error", err, "consul_addr", h.config.Addr)
			//TODO(schmichael) Soft fail?
			return nil
		}

		destAddr = net.JoinHostPort(destURL.Hostname(), "8502")
	}

	//TODO(schmichael) Does the proxy need to handle TLS or does envoy
	//bootstrap set it up? Check {Enable,Verify}SSL, CAFile, CertFile,
	//KeyFile, etc.
	hostGRPCSockPath := filepath.Join(h.allocDir.AllocDir, allocdir.TaskGRPCSocket)
	listener, err := net.Listen("unix", hostGRPCSockPath)
	if err != nil {
		h.logger.Warn("unable to create unix sockets for Consul", "error", err, "grpc_socket", hostGRPCSockPath)
		//TODO(schmichael) Soft fail?
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	h.cancel = func() {
		cancel()
		listener.Close()
	}
	go proxy(ctx, h.logger, destAddr, listener)
	return nil
}

func (h *consulSockHook) Postrun() error {
	if h.cancel != nil {
		h.cancel()
	}
	return nil
}

func proxy(pctx context.Context, logger hclog.Logger, dest string, l net.Listener) {
	ctx, cancel := context.WithCancel(pctx)
	defer func() {
		cancel()
		l.Close()
	}()
	for ctx.Err() == nil {
		conn, err := l.Accept()
		if err != nil {
			// Debug level because it's likely due to a shutdown
			//TODO(schmichael) squelch logging entirely?
			logger.Debug("proxy accept error", "error", err)
			return
		}

		go proxyConn(ctx, logger, dest, conn)
	}
}

func proxyConn(ctx context.Context, logger hclog.Logger, destAddr string, conn net.Conn) {
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

	// socket -> gRPC
	go func() {
		defer dest.Close()
		defer conn.Close()
		n, err := io.Copy(dest, conn)
		if err != nil {
			logger.Error("error proxying to Consul", "error", err, "dest", destAddr,
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
	go func() {
		defer dest.Close()
		defer conn.Close()
		n, err := io.Copy(conn, dest)
		if err != nil {
			logger.Trace("error proxying from Consul", "error", err, "dest", destAddr,
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

	// Closer goroutine: close connections to break out of copies.
	go func() {
		<-ctx.Done()
		conn.Close()
		dest.Close()
	}()
}
