package config

import (
	"reflect"
	"testing"
	"time"
)

func TestAutopilotConfig_Merge(t *testing.T) {
	trueValue, falseValue := true, false

	c1 := &AutopilotConfig{
		CleanupDeadServers:      &falseValue,
		ServerStabilizationTime: 1 * time.Second,
		LastContactThreshold:    1 * time.Second,
		MaxTrailingLogs:         1,
		RedundancyZoneTag:       "1",
		DisableUpgradeMigration: &falseValue,
		UpgradeVersionTag:       "1",
	}

	c2 := &AutopilotConfig{
		CleanupDeadServers:      &trueValue,
		ServerStabilizationTime: 2 * time.Second,
		LastContactThreshold:    2 * time.Second,
		MaxTrailingLogs:         2,
		RedundancyZoneTag:       "2",
		DisableUpgradeMigration: nil,
		UpgradeVersionTag:       "2",
	}

	e := &AutopilotConfig{
		CleanupDeadServers:      &trueValue,
		ServerStabilizationTime: 2 * time.Second,
		LastContactThreshold:    2 * time.Second,
		MaxTrailingLogs:         2,
		RedundancyZoneTag:       "2",
		DisableUpgradeMigration: &falseValue,
		UpgradeVersionTag:       "2",
	}

	result := c1.Merge(c2)
	if !reflect.DeepEqual(result, e) {
		t.Fatalf("bad:\n%#v\n%#v", result, e)
	}
}
