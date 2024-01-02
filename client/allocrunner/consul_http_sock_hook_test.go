// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocrunner

import (
	"bytes"
	"net"
	"path/filepath"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/stretchr/testify/require"
)

func TestConsulSocketHook_PrerunPostrun_Ok(t *testing.T) {
	ci.Parallel(t)

	fakeConsul, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer fakeConsul.Close()

	consulConfig := &config.ConsulConfig{
		Addr: fakeConsul.Addr().String(),
	}

	alloc := mock.ConnectNativeAlloc("bridge")

	logger := testlog.HCLogger(t)

	allocDir, cleanupDir := allocdir.TestAllocDir(t, logger, "ConnectNativeTask", alloc.ID)
	defer cleanupDir()

	// start unix socket proxy
	h := newConsulHTTPSocketHook(logger, alloc, allocDir, consulConfig)
	require.NoError(t, h.Prerun())

	httpSocket := filepath.Join(allocDir.AllocDir, allocdir.AllocHTTPSocket)
	taskCon, err := net.Dial("unix", httpSocket)
	require.NoError(t, err)

	// write to consul from task to ensure data is proxied out of the netns
	input := bytes.Repeat([]byte{'X'}, 5*1024)
	errCh := make(chan error, 1)
	go func() {
		_, err := taskCon.Write(input)
		errCh <- err
	}()

	// accept the connection from inside the netns
	consulConn, err := fakeConsul.Accept()
	require.NoError(t, err)
	defer consulConn.Close()

	output := make([]byte, len(input))
	_, err = consulConn.Read(output)
	require.NoError(t, err)
	require.NoError(t, <-errCh)
	require.Equal(t, input, output)

	// read from consul to ensure http response bodies can come back
	input = bytes.Repeat([]byte{'Y'}, 5*1024)
	go func() {
		_, err := consulConn.Write(input)
		errCh <- err
	}()

	output = make([]byte, len(input))
	_, err = taskCon.Read(output)
	require.NoError(t, err)
	require.NoError(t, <-errCh)
	require.Equal(t, input, output)

	// stop the unix socket proxy
	require.NoError(t, h.Postrun())

	// consul reads should now error
	n, err := consulConn.Read(output)
	require.Error(t, err)
	require.Zero(t, n)

	// task reads and writes should error
	n, err = taskCon.Write(input)
	require.Error(t, err)
	require.Zero(t, n)
	n, err = taskCon.Read(output)
	require.Error(t, err)
	require.Zero(t, n)
}

func TestConsulHTTPSocketHook_Prerun_Error(t *testing.T) {
	ci.Parallel(t)

	logger := testlog.HCLogger(t)

	consulConfig := new(config.ConsulConfig)

	alloc := mock.Alloc()
	connectNativeAlloc := mock.ConnectNativeAlloc("bridge")

	allocDir, cleanupDir := allocdir.TestAllocDir(t, logger, "ConnectNativeTask", alloc.ID)
	defer cleanupDir()

	{
		// an alloc without a connect native task should not return an error
		h := newConsulHTTPSocketHook(logger, alloc, allocDir, consulConfig)
		require.NoError(t, h.Prerun())

		// postrun should be a noop
		require.NoError(t, h.Postrun())
	}

	{
		// an alloc with a native task should return an error when consul is not
		// configured
		h := newConsulHTTPSocketHook(logger, connectNativeAlloc, allocDir, consulConfig)
		require.EqualError(t, h.Prerun(), "consul address must be set on nomad client")

		// Postrun should be a noop
		require.NoError(t, h.Postrun())
	}
}
