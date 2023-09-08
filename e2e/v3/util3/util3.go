// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package util3

import (
	"fmt"
	"math/rand"
	"os"
	"testing"
)

func ShortID(prefix string) string {
	num := rand.Int31() % 1000
	return fmt.Sprintf("%s-%03d", prefix, num)
}

// Log3 is a helper for verbose logging in e2e/v3 packages.
//
// Do not call this directly from tests.
func Log3(t *testing.T, verbose bool, msg string, args ...any) {
	env := os.Getenv("NOMAD_E2E_VERBOSE")
	on := verbose || env == "1" || env == "true"
	if on {
		t.Logf(msg, args...)
	}
}
