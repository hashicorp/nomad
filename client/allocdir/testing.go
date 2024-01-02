// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocdir

import (
	"os"

	hclog "github.com/hashicorp/go-hclog"
	testing "github.com/mitchellh/go-testing-interface"
)

// TestAllocDir returns a built alloc dir in a temporary directory and cleanup
// func.
func TestAllocDir(t testing.T, l hclog.Logger, prefix, id string) (*AllocDir, func()) {
	dir, err := os.MkdirTemp("", prefix)
	if err != nil {
		t.Fatalf("Couldn't create temp dir: %v", err)
	}

	allocDir := NewAllocDir(l, dir, id)

	cleanup := func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Logf("error cleaning up alloc dir %q: %v", prefix, err)
		}

		if err := allocDir.Destroy(); err != nil {
			t.Logf("error cleaning up alloc dir %q: %v", prefix, err)
		}
	}

	if err := allocDir.Build(); err != nil {
		cleanup()
		t.Fatalf("error building alloc dir %q: %v", prefix, err)
	}

	return allocDir, cleanup
}
