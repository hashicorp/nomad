// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package numalib

import (
	"testing"

	"github.com/shoenig/test/must"
)

func Test_NoImpl_yes(t *testing.T) {
	original := new(Topology)
	fallback := NoImpl(original)
	must.NotEqOp(t, original, fallback) // pointer is replaced
	must.Len(t, 1, fallback.Cores)
}

func Test_NoImpl_no(t *testing.T) {
	original := Scan(PlatformScanners())
	fallback := NoImpl(original)
	must.EqOp(t, original, fallback) // pointer is same
}
