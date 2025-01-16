// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"github.com/hashicorp/go-set/v3"
)

// validLogLevels is the set of log level values that are valid for a Nomad
// agent.
var validLogLevels = set.From([]string{"TRACE", "DEBUG", "INFO", "WARN", "ERROR", "OFF"})

// isLogLevelValid returns whether the passed agent log level is valid.
func isLogLevelValid(level string) bool { return validLogLevels.Contains(level) }
