// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package docker

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/testutil"
	"github.com/shoenig/test/must"
)

func Test_dockerCgroup(t *testing.T) {
	testutil.RequireRoot(t)

	ci.Parallel(t)

	t.Run("preset", func(t *testing.T) {
		testutil.CgroupsCompatible(t)

		h := new(taskHandle)
		h.containerCgroup = "/some/preset"
		result := h.dockerCgroup()
		must.Eq(t, "/some/preset", result)
	})

	t.Run("v1", func(t *testing.T) {
		testutil.CgroupsCompatibleV1(t)
		h := new(taskHandle)
		h.containerID = "abc123"
		result := h.dockerCgroup()
		must.Eq(t, "/sys/fs/cgroup/cpuset/docker/abc123", result)
	})

	t.Run("v2-systemd", func(t *testing.T) {
		testutil.CgroupsCompatibleV2(t)
		h := new(taskHandle)
		h.containerID = "abc123"
		result := h.dockerCgroup()
		must.Eq(t, "/sys/fs/cgroup/system.slice/docker-abc123.scope", result)
	})

	t.Run("v2-cgroupfs", func(t *testing.T) {
		testutil.CgroupsCompatibleV2(t)
		h := new(taskHandle)
		h.containerID = "abc123"
		h.dockerCGroupDriver = "cgroupfs"
		result := h.dockerCgroup()
		must.Eq(t, "/sys/fs/cgroup/docker/abc123", result)
	})
}
