// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package docker

import (
	"context"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/shoenig/test/must"
)

// TestDockerDriver_FingerprintHealth asserts that docker reports healthy
// whenever Docker is supported.
//
// In Linux CI and AppVeyor Windows environment, it should be enabled.
func TestDockerDriver_FingerprintHealth(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d := NewDockerDriver(ctx, top, testlog.HCLogger(t)).(*Driver)

	fp := d.buildFingerprint()
	must.Eq(t, drivers.HealthStateHealthy, fp.Health)
}

// TestDockerDriver_NonRoot_CGV2 tests that the docker drivers is not enabled
// when running as a non-root user on a machine with a v2 cgroups controller.
func TestDockerDriver_NonRoot_CGV2(t *testing.T) {
	ci.Parallel(t)
	testutil.DockerCompatible(t)
	testutil.CgroupsCompatibleV2(t)
	testutil.RequireNonRoot(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d := NewDockerDriver(ctx, top, testlog.HCLogger(t)).(*Driver)

	fp := d.buildFingerprint()
	must.Eq(t, drivers.HealthStateUndetected, fp.Health)
	must.Eq(t, drivers.DriverRequiresRootMessage, fp.HealthDescription)
}
