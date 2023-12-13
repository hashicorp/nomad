// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocrunner

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"path/filepath"
	"sync"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConsulGRPCSocketHook_PrerunPostrun_Ok asserts that a proxy is started when the
// Consul unix socket hook's Prerun method is called and stopped with the
// Postrun method is called.
func TestConsulGRPCSocketHook_PrerunPostrun_Ok(t *testing.T) {
	ci.Parallel(t)

	// As of Consul 1.6.0 the test server does not support the gRPC
	// endpoint so we have to fake it.
	fakeConsul, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer fakeConsul.Close()
	consulConfig := &config.ConsulConfig{
		GRPCAddr: fakeConsul.Addr().String(),
	}

	alloc := mock.ConnectAlloc()

	logger := testlog.HCLogger(t)

	allocDir, cleanup := allocdir.TestAllocDir(t, logger, "EnvoyBootstrap", alloc.ID)
	defer cleanup()

	// Start the unix socket proxy
	h := newConsulGRPCSocketHook(logger, alloc, allocDir, consulConfig, map[string]string{})
	require.NoError(t, h.Prerun())

	gRPCSock := filepath.Join(allocDir.AllocDir, allocdir.AllocGRPCSocket)
	envoyConn, err := net.Dial("unix", gRPCSock)
	require.NoError(t, err)

	// Write to Consul to ensure data is proxied out of the netns
	input := bytes.Repeat([]byte{'X'}, 5*1024)
	errCh := make(chan error, 1)
	go func() {
		_, err := envoyConn.Write(input)
		errCh <- err
	}()

	// Accept the connection from the netns
	consulConn, err := fakeConsul.Accept()
	require.NoError(t, err)
	defer consulConn.Close()

	output := make([]byte, len(input))
	_, err = consulConn.Read(output)
	require.NoError(t, err)
	require.NoError(t, <-errCh)
	require.Equal(t, input, output)

	// Read from Consul to ensure data is proxied into the netns
	input = bytes.Repeat([]byte{'Y'}, 5*1024)
	go func() {
		_, err := consulConn.Write(input)
		errCh <- err
	}()

	_, err = envoyConn.Read(output)
	require.NoError(t, err)
	require.NoError(t, <-errCh)
	require.Equal(t, input, output)

	// Stop the unix socket proxy
	require.NoError(t, h.Postrun())

	// Consul reads should error
	n, err := consulConn.Read(output)
	require.Error(t, err)
	require.Zero(t, n)

	// Envoy reads and writes should error
	n, err = envoyConn.Write(input)
	require.Error(t, err)
	require.Zero(t, n)
	n, err = envoyConn.Read(output)
	require.Error(t, err)
	require.Zero(t, n)
}

// TestConsulGRPCSocketHook_Prerun_Error asserts that invalid Consul addresses cause
// Prerun to return an error if the alloc requires a grpc proxy.
func TestConsulGRPCSocketHook_Prerun_Error(t *testing.T) {
	ci.Parallel(t)

	logger := testlog.HCLogger(t)

	// A config without an Addr or GRPCAddr is invalid.
	consulConfig := &config.ConsulConfig{}

	alloc := mock.Alloc()
	connectAlloc := mock.ConnectAlloc()

	allocDir, cleanup := allocdir.TestAllocDir(t, logger, "EnvoyBootstrap", alloc.ID)
	defer cleanup()

	{
		// An alloc without a Connect proxy sidecar should not return
		// an error.
		h := newConsulGRPCSocketHook(logger, alloc, allocDir, consulConfig, map[string]string{})
		require.NoError(t, h.Prerun())

		// Postrun should be a noop
		require.NoError(t, h.Postrun())
	}

	{
		// An alloc *with* a Connect proxy sidecar *should* return an error
		// when Consul is not configured.
		h := newConsulGRPCSocketHook(logger, connectAlloc, allocDir, consulConfig, map[string]string{})
		require.EqualError(t, h.Prerun(), "consul address must be set on nomad client")

		// Postrun should be a noop
		require.NoError(t, h.Postrun())
	}

	{
		// Updating an alloc without a sidecar to have a sidecar should
		// error when the sidecar is added.
		h := newConsulGRPCSocketHook(logger, alloc, allocDir, consulConfig, map[string]string{})
		require.NoError(t, h.Prerun())

		req := &interfaces.RunnerUpdateRequest{
			Alloc: connectAlloc,
		}
		require.EqualError(t, h.Update(req), "consul address must be set on nomad client")

		// Postrun should be a noop
		require.NoError(t, h.Postrun())
	}
}

// TestConsulGRPCSocketHook_proxy_Unix asserts that the destination can be a unix
// socket path.
func TestConsulGRPCSocketHook_proxy_Unix(t *testing.T) {
	ci.Parallel(t)

	dir := t.TempDir()

	// Setup fake listener that would be inside the netns (normally a unix
	// socket, but it doesn't matter for this test).
	src, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer src.Close()

	// Setup fake listener that would be Consul outside the netns. Use a
	// socket as Consul may be configured to listen on a unix socket.
	destFn := filepath.Join(dir, "fakeconsul.sock")
	dest, err := net.Listen("unix", destFn)
	require.NoError(t, err)
	defer dest.Close()

	// Collect errors (must have len > goroutines)
	errCh := make(chan error, 10)

	// Block until completion
	wg := sync.WaitGroup{}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wg.Add(1)
	go func() {
		defer wg.Done()
		proxy(ctx, testlog.HCLogger(t), "unix://"+destFn, src)
	}()

	// Fake Envoy
	// Connect and write to the src (netns) side of the proxy; then read
	// and exit.
	wg.Add(1)
	go func() {
		defer func() {
			// Cancel after final read has completed (or an error
			// has occurred)
			cancel()

			wg.Done()
		}()

		addr := src.Addr()
		conn, err := net.Dial(addr.Network(), addr.String())
		if err != nil {
			errCh <- err
			return
		}

		defer conn.Close()

		if _, err := conn.Write([]byte{'X'}); err != nil {
			errCh <- err
			return
		}

		recv := make([]byte, 1)
		if _, err := conn.Read(recv); err != nil {
			errCh <- err
			return
		}

		if expected := byte('Y'); recv[0] != expected {
			errCh <- fmt.Errorf("expected %q but received: %q", expected, recv[0])
			return
		}
	}()

	// Fake Consul on a unix socket
	// Listen, receive 1 byte, write a response, and exit
	wg.Add(1)
	go func() {
		defer wg.Done()

		conn, err := dest.Accept()
		if err != nil {
			errCh <- err
			return
		}

		// Close listener now. No more connections expected.
		if err := dest.Close(); err != nil {
			errCh <- err
			return
		}

		defer conn.Close()

		recv := make([]byte, 1)
		if _, err := conn.Read(recv); err != nil {
			errCh <- err
			return
		}

		if expected := byte('X'); recv[0] != expected {
			errCh <- fmt.Errorf("expected %q but received: %q", expected, recv[0])
			return
		}

		if _, err := conn.Write([]byte{'Y'}); err != nil {
			errCh <- err
			return
		}
	}()

	// Wait for goroutines to complete
	wg.Wait()

	// Make sure no errors occurred
	for len(errCh) > 0 {
		assert.NoError(t, <-errCh)
	}
}
