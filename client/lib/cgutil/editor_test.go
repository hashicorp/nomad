// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build linux

package cgutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/shoenig/test/must"
)

func createCG(t *testing.T) (string, func()) {
	name := uuid.Short() + ".scope"
	path := filepath.Join(CgroupRoot, name)
	err := os.Mkdir(path, 0o755)
	must.NoError(t, err)

	return name, func() {
		_ = os.Remove(path)
	}
}

func TestCG_editor(t *testing.T) {
	testutil.CgroupsCompatibleV2(t)

	cg, rm := createCG(t)
	t.Cleanup(rm)

	edits := &editor{cg}
	writeErr := edits.write("cpu.weight.nice", "13")
	must.NoError(t, writeErr)

	b, readErr := edits.read("cpu.weight.nice")
	must.NoError(t, readErr)
	must.Eq(t, "13", b)
}
