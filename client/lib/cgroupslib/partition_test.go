// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package cgroupslib

import (
	"path/filepath"
	"testing"

	"github.com/hashicorp/nomad/client/lib/idset"
	"github.com/shoenig/test/must"
)

// testPartition creates a fresh partition configured with cores 10-20.
func testPartition(t *testing.T) *partition {
	dir := t.TempDir()
	shareFile := filepath.Join(dir, "share.cpus")
	reserveFile := filepath.Join(dir, "reserve.cpus")
	return &partition{
		sharePath:   shareFile,
		reservePath: reserveFile,
		share:       idset.From[idset.CoreID]([]idset.CoreID{10, 11, 12, 13, 14, 15, 16, 17, 18, 19}),
		reserve:     idset.Empty[idset.CoreID](),
	}
}

func coreset(ids ...idset.CoreID) *idset.Set[idset.CoreID] {
	return idset.From[idset.CoreID](ids)
}

func TestPartition_Restore(t *testing.T) {
	p := testPartition(t)

	must.NotEmpty(t, p.share)
	must.Empty(t, p.reserve)

	p.Restore(coreset(11, 13))
	p.Restore(coreset(15, 16, 17))
	p.Restore(coreset(10, 19))

	expShare := idset.From[idset.CoreID]([]idset.CoreID{12, 14, 18})
	expReserve := idset.From[idset.CoreID]([]idset.CoreID{11, 13, 15, 16, 17, 10, 19})

	must.Eq(t, expShare, p.share)
	must.Eq(t, expReserve, p.reserve)

	// restore does not write to the cgroup interface
	must.FileNotExists(t, p.sharePath)
	must.FileNotExists(t, p.reservePath)
}

func TestPartition_Reserve(t *testing.T) {
	p := testPartition(t)

	p.Reserve(coreset(10, 15, 19))
	p.Reserve(coreset(12, 13))

	expShare := idset.From[idset.CoreID]([]idset.CoreID{11, 14, 16, 17, 18})
	expReserve := idset.From[idset.CoreID]([]idset.CoreID{10, 12, 13, 15, 19})

	must.Eq(t, expShare, p.share)
	must.Eq(t, expReserve, p.reserve)

	must.FileContains(t, p.sharePath, "11,14,16-18")
	must.FileContains(t, p.reservePath, "10,12-13,15,19")
}

func TestPartition_Release(t *testing.T) {
	p := testPartition(t)

	// some reservations
	p.Reserve(coreset(10, 15, 19))
	p.Reserve(coreset(12, 13))
	p.Reserve(coreset(11, 18))

	must.FileContains(t, p.sharePath, "14,16-17")
	must.FileContains(t, p.reservePath, "10-13,15,18-19")

	// release 1
	p.Release(coreset(12, 13))
	must.FileContains(t, p.sharePath, "12-14,16-17")
	must.FileContains(t, p.reservePath, "10-11,15,18-19")

	// release 2
	p.Release(coreset(10, 15, 19))
	must.FileContains(t, p.sharePath, "10,12-17,19")
	must.FileContains(t, p.reservePath, "11,18")

	// release 3
	p.Release(coreset(11, 18))
	must.FileContains(t, p.sharePath, "10-19")
	must.FileContains(t, p.reservePath, "")
}
