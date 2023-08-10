// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package numalib

import (
	"testing"

	"github.com/shoenig/test/must"
)

// TestScanTopology is going to be different on every machine; even the CI
// systems change sometimes so it's hard to make good assertions here.
func TestScanTopology(t *testing.T) {
	top := Scan(PlatformScanners())
	must.Positive(t, top.UsableCompute())
	must.Positive(t, top.TotalCompute())
	must.Positive(t, top.NumCores())
}
