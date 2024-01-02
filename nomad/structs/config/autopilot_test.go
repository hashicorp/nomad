// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package config

import (
	"reflect"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
)

func TestAutopilotConfig_Merge(t *testing.T) {
	ci.Parallel(t)

	trueValue, falseValue := true, false

	c1 := &AutopilotConfig{
		CleanupDeadServers:      &falseValue,
		ServerStabilizationTime: 1 * time.Second,
		LastContactThreshold:    1 * time.Second,
		MaxTrailingLogs:         1,
		MinQuorum:               1,
		EnableRedundancyZones:   &trueValue,
		DisableUpgradeMigration: &falseValue,
		EnableCustomUpgrades:    &trueValue,
	}

	c2 := &AutopilotConfig{
		CleanupDeadServers:      &trueValue,
		ServerStabilizationTime: 2 * time.Second,
		LastContactThreshold:    2 * time.Second,
		MaxTrailingLogs:         2,
		MinQuorum:               2,
		EnableRedundancyZones:   nil,
		DisableUpgradeMigration: nil,
		EnableCustomUpgrades:    nil,
	}

	e := &AutopilotConfig{
		CleanupDeadServers:      &trueValue,
		ServerStabilizationTime: 2 * time.Second,
		LastContactThreshold:    2 * time.Second,
		MaxTrailingLogs:         2,
		MinQuorum:               2,
		EnableRedundancyZones:   &trueValue,
		DisableUpgradeMigration: &falseValue,
		EnableCustomUpgrades:    &trueValue,
	}

	result := c1.Merge(c2)
	if !reflect.DeepEqual(result, e) {
		t.Fatalf("bad:\n%#v\n%#v", result, e)
	}
}
