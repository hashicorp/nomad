// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package config

import (
	"testing"

	"github.com/hashicorp/nomad/v2/ci"
	"github.com/shoenig/test/must"
)

func TestReporting_Merge(t *testing.T) {
	ci.Parallel(t)

	a := &ReportingConfig{
		License: &LicenseReportingConfig{
			Enabled: new(false),
		},
	}

	b := &ReportingConfig{
		License: &LicenseReportingConfig{
			Enabled: new(true),
		},
	}

	res := a.Merge(b)
	must.True(t, *res.License.Enabled)

	res = res.Merge(a)
	must.False(t, *res.License.Enabled)
}
