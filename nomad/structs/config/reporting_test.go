// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package config

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/shoenig/test/must"
)

func TestReporting_Merge(t *testing.T) {
	ci.Parallel(t)

	a := &Reporting{
		License: &LicenseConfig{
			Enabled: pointer.Of(false),
		},
	}

	b := &Reporting{
		License: &LicenseConfig{
			Enabled: pointer.Of(true),
		},
	}

	res := a.Merge(b)
	must.True(t, *res.License.Enabled)

	res = res.Merge(a)
	must.False(t, *res.License.Enabled)
}
