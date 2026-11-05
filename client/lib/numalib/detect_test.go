// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package numalib

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/lib/idset"
	"github.com/hashicorp/nomad/client/lib/numalib/hw"
	"github.com/shoenig/test/must"
)

// TestScanTopology is going to be different on every machine; even the CI
// systems change sometimes so it's hard to make good assertions here.
func TestScanTopology(t *testing.T) {
	top := Scan(PlatformScanners(false))
	must.Positive(t, top.UsableCompute())
	must.Positive(t, top.TotalCompute())
	must.Positive(t, top.NumCores())
}

func TestConfigScanner_OverrideCoreSpeeds(t *testing.T) {
	ci.Parallel(t)

	const (
		pCore = 4000
		eCore = 3000
	)
	for _, tc := range []struct {
		name      string
		scanner   *ConfigScanner
		expectedP hw.MHz
		expectedE hw.MHz
	}{
		{
			name: "no overrides",
			scanner: &ConfigScanner{
				ReservedCores: idset.Empty[hw.CoreID](),
			},
			expectedP: pCore,
			expectedE: eCore,
		},
		{
			name: "override cpu_total_compute",
			scanner: &ConfigScanner{
				TotalCompute:  hw.MHz(1000),
				ReservedCores: idset.Empty[hw.CoreID](),
			},
			expectedP: 500,
			expectedE: 500,
		},
		{
			name: "override cpu_total_compute rounds",
			scanner: &ConfigScanner{
				TotalCompute:  hw.MHz(1111),
				ReservedCores: idset.Empty[hw.CoreID](),
			},
			expectedP: 555,
			expectedE: 555,
		},
		{
			name: "override cpu_total_compute can zero core mhz",
			scanner: &ConfigScanner{
				TotalCompute:  hw.MHz(1),
				ReservedCores: idset.Empty[hw.CoreID](),
			},
			expectedP: 0,
			expectedE: 0,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// 2 core system
			top := &Topology{Cores: []Core{
				{
					ID:        1,
					Grade:     Performance,
					BaseSpeed: pCore,
				},
				{
					ID:        2,
					Grade:     Efficiency,
					BaseSpeed: eCore,
				},
			}}

			tc.scanner.ScanSystem(top)
			p, e := top.CoreSpeeds()
			must.Eq(t, tc.expectedP, p)
			must.Eq(t, tc.expectedE, e)
		})
	}
}
