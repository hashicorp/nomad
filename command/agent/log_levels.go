// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"io"

	"github.com/hashicorp/logutils"
)

// LevelFilter returns a LevelFilter that is configured with the log
// levels that we use.
func LevelFilter() *logutils.LevelFilter {
	return &logutils.LevelFilter{
		Levels:   []logutils.LogLevel{"TRACE", "DEBUG", "INFO", "WARN", "ERROR", "OFF"},
		MinLevel: "INFO",
		Writer:   io.Discard,
	}
}

// ValidateLevelFilter verifies that the log levels within the filter
// are valid.
func ValidateLevelFilter(minLevel logutils.LogLevel, filter *logutils.LevelFilter) bool {
	for _, level := range filter.Levels {
		if level == minLevel {
			return true
		}
	}
	return false
}
