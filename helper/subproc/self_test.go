// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package subproc

import (
	"testing"

	"github.com/shoenig/test/must"
)

func TestSelf(t *testing.T) {
	value := Self()
	must.NotEq(t, "", value)
}
