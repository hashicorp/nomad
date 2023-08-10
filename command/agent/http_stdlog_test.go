// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"bytes"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/ci"
	"github.com/stretchr/testify/require"
)

func TestHttpServerLoggerFilters_Level_Info(t *testing.T) {
	ci.Parallel(t)

	var buf bytes.Buffer
	hclogger := hclog.New(&hclog.LoggerOptions{
		Name:   "testlog",
		Output: &buf,
		Level:  hclog.Info,
	})

	stdlogger := newHTTPServerLogger(hclogger)

	// spurious logging would be filtered out
	stdlogger.Printf("spurious logging: %v", "arg")
	require.Empty(t, buf.String())

	// panics are included
	stdlogger.Printf("panic while processing: %v", "endpoint")
	require.Contains(t, buf.String(), "[ERROR] testlog: panic while processing: endpoint")

}

func TestHttpServerLoggerFilters_Level_Trace(t *testing.T) {
	ci.Parallel(t)

	var buf bytes.Buffer
	hclogger := hclog.New(&hclog.LoggerOptions{
		Name:   "testlog",
		Output: &buf,
		Level:  hclog.Trace,
	})

	stdlogger := newHTTPServerLogger(hclogger)

	// spurious logging will be included as Trace level
	stdlogger.Printf("spurious logging: %v", "arg")
	require.Contains(t, buf.String(), "[TRACE] testlog: spurious logging: arg")

	stdlogger.Printf("panic while processing: %v", "endpoint")
	require.Contains(t, buf.String(), "[ERROR] testlog: panic while processing: endpoint")

}
