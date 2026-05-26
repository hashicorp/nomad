// Copyright IBM Corp. 2015, 2025
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
