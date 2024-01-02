// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"bytes"
	"log"

	hclog "github.com/hashicorp/go-hclog"
)

func newHTTPServerLogger(logger hclog.Logger) *log.Logger {
	return log.New(&httpServerLoggerAdapter{logger}, "", 0)
}

// a logger adapter that forwards http server logs as a Trace level
// hclog log entries. Logs related to panics are forwarded with Error level.
//
// HTTP server logs are typically spurious as they represent HTTP
// client errors (e.g. TLS handshake failures).
type httpServerLoggerAdapter struct {
	logger hclog.Logger
}

func (l *httpServerLoggerAdapter) Write(data []byte) (int, error) {
	if bytes.Contains(data, []byte("panic")) {
		str := string(bytes.TrimRight(data, " \t\n"))
		l.logger.Error(str)
	} else if l.logger.IsTrace() {
		str := string(bytes.TrimRight(data, " \t\n"))
		l.logger.Trace(str)
	}

	return len(data), nil
}
