// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package useragent

import (
	"testing"
)

func TestUserAgent(t *testing.T) {
	projectURL = "https://nomad-test.com"
	rt = "go5.0"
	versionFunc = func() string { return "1.2.3" }

	act := String()

	exp := "Nomad/1.2.3 (+https://nomad-test.com; go5.0)"
	if exp != act {
		t.Errorf("expected %q to be %q", act, exp)
	}
}
