// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package cgroupslib

import (
	"testing"

	"github.com/shoenig/test/must"
)

func Test_MaybeDisableMemorySwappiness(t *testing.T) {
	disable := MaybeDisableMemorySwappiness()
	var zero = uint64(0)
	must.Eq(t, &zero, disable)
}
