// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consulcompat

import (
	"os"
	"syscall"
	"testing"
)

const (
	envTempDir = "NOMAD_E2E_CONSULCOMPAT_BASEDIR"
	envGate    = "NOMAD_E2E_CONSULCOMPAT"
)

func TestConsulCompat(t *testing.T) {
	if os.Getenv(envGate) != "1" {
		t.Skip(envGate + " is not set; skipping")
	}
	if syscall.Geteuid() != 0 {
		t.Skip("must be run as root so that clients can run Docker tasks")
	}

	t.Run("testConsulVersions", func(t *testing.T) {
		baseDir := os.Getenv(envTempDir)
		if baseDir == "" {
			baseDir = t.TempDir()
		}

		versions := scanConsulVersions(t, getMinimumVersion(t))
		versions.ForEach(func(b build) bool {
			downloadConsulBuild(t, b, baseDir)

			testConsulBuildLegacy(t, b, baseDir)
			testConsulBuild(t, b, baseDir)
			return true
		})
	})
}
