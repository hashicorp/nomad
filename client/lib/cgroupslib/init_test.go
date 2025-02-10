// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package cgroupslib

import (
	"testing"

	"github.com/shoenig/test/must"
)

func TestCustomPathCG1(t *testing.T) {
	exp := "/sys/fs/cgroup/pids/custom/path"

	t.Run("absolute", func(t *testing.T) {
		result := CustomPathCG1("pids", "/sys/fs/cgroup/pids/custom/path")
		must.Eq(t, result, exp)
	})

	t.Run("relative", func(t *testing.T) {
		result := CustomPathCG1("pids", "custom/path")
		must.Eq(t, result, exp)
	})
}

func TestCustomPathCG2(t *testing.T) {
	exp := "/sys/fs/cgroup/custom.slice/path.scope"

	t.Run("unset", func(t *testing.T) {
		result := CustomPathCG2("")
		must.Eq(t, result, "")
	})

	t.Run("absolute", func(t *testing.T) {
		result := CustomPathCG2("/sys/fs/cgroup/custom.slice/path.scope")
		must.Eq(t, result, exp)
	})

	t.Run("relative", func(t *testing.T) {
		result := CustomPathCG2("custom.slice/path.scope")
		must.Eq(t, result, exp)
	})
}
