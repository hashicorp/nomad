// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package cgroupslib

import (
	"path/filepath"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/lib/idset"
	"github.com/hashicorp/nomad/client/lib/numalib/hw"
	"github.com/shoenig/test/must"
)

// testPartition creates a fresh partition configured with cores 10-19.
func testPartition(t *testing.T) *partition {
	dir := t.TempDir()
	shareFile := filepath.Join(dir, "share.cpus")
	reserveFile := filepath.Join(dir, "reserve.cpus")
	return &partition{
		log:          hclog.NewNullLogger(),
		usableCores:  idset.From[hw.CoreID]([]hw.CoreID{10, 11, 12, 13, 14, 15, 16, 17, 18, 19}),
		sharePath:    shareFile,
		reservePath:  reserveFile,
		reservations: make(map[string]*idset.Set[hw.CoreID]),
	}
}

func coreset(ids ...hw.CoreID) *idset.Set[hw.CoreID] {
	return idset.From[hw.CoreID](ids)
}

func TestPartition_Restore(t *testing.T) {
	p := testPartition(t)

	must.NotEmpty(t, p.shared())
	must.Empty(t, p.reserved())

	p.Restore("a", coreset(11, 13))
	p.Restore("b", coreset(15, 16, 17))
	p.Restore("c", coreset(10, 19))

	expShare := idset.From[hw.CoreID]([]hw.CoreID{12, 14, 18})
	expReserve := idset.From[hw.CoreID]([]hw.CoreID{11, 13, 15, 16, 17, 10, 19})

	must.Eq(t, expShare, p.shared())
	must.Eq(t, expReserve, p.reserved())

	// restore does not write to the cgroup interface
	must.FileNotExists(t, p.sharePath)
	must.FileNotExists(t, p.reservePath)
}

func TestPartition_Restore_multipleTasks(t *testing.T) {
	p := testPartition(t)

	// each task of an alloc is restored separately; the alloc holds the union
	p.Restore("a", coreset(11, 13))
	p.Restore("a", coreset(15))
	p.Restore("b", coreset())

	must.Eq(t, coreset(11, 13, 15), p.reserved())
	must.MapContainsKey(t, p.reservations, "a")
	must.MapNotContainsKey(t, p.reservations, "b")
}

func TestPartition_Reserve(t *testing.T) {
	p := testPartition(t)

	must.NoError(t, p.Reserve("a", coreset(10, 15, 19)))
	must.NoError(t, p.Reserve("b", coreset(12, 13)))

	expShare := idset.From[hw.CoreID]([]hw.CoreID{11, 14, 16, 17, 18})
	expReserve := idset.From[hw.CoreID]([]hw.CoreID{10, 12, 13, 15, 19})

	must.Eq(t, expShare, p.shared())
	must.Eq(t, expReserve, p.reserved())

	must.FileContains(t, p.sharePath, "11,14,16-18")
	must.FileContains(t, p.reservePath, "10,12-13,15,19")
}

func TestPartition_Reserve_unusableCores(t *testing.T) {
	p := testPartition(t)

	// cores outside the usable set are ignored
	must.NoError(t, p.Reserve("a", coreset(18, 19, 20, 21)))
	must.FileContains(t, p.sharePath, "10-17")
	must.FileContains(t, p.reservePath, "18-19")

	// an alloc with no usable cores is not tracked
	must.NoError(t, p.Reserve("b", coreset()))
	must.NoError(t, p.Reserve("c", coreset(20, 21)))
	must.MapLen(t, 1, p.reservations)
}

func TestPartition_Reserve_afterRestore(t *testing.T) {
	p := testPartition(t)

	// a restored alloc runs its prerun hooks again; the books do not change
	p.Restore("a", coreset(10, 11))
	must.NoError(t, p.Reserve("a", coreset(10, 11)))

	must.MapLen(t, 1, p.reservations)
	must.FileContains(t, p.sharePath, "12-19")
	must.FileContains(t, p.reservePath, "10-11")

	must.NoError(t, p.Release("a"))
	must.FileContains(t, p.sharePath, "10-19")
	must.FileContains(t, p.reservePath, "")
}

func TestPartition_Release(t *testing.T) {
	p := testPartition(t)

	// some reservations
	must.NoError(t, p.Reserve("a", coreset(10, 15, 19)))
	must.NoError(t, p.Reserve("b", coreset(12, 13)))
	must.NoError(t, p.Reserve("c", coreset(11, 18)))

	must.FileContains(t, p.sharePath, "14,16-17")
	must.FileContains(t, p.reservePath, "10-13,15,18-19")

	// release 1
	must.NoError(t, p.Release("b"))
	must.FileContains(t, p.sharePath, "12-14,16-17")
	must.FileContains(t, p.reservePath, "10-11,15,18-19")

	// release 2
	must.NoError(t, p.Release("a"))
	must.FileContains(t, p.sharePath, "10,12-17,19")
	must.FileContains(t, p.reservePath, "11,18")

	// release 3
	must.NoError(t, p.Release("c"))
	must.FileContains(t, p.sharePath, "10-19")
	must.FileContains(t, p.reservePath, "")
	must.MapEmpty(t, p.reservations)

	// releasing again is a no-op
	must.NoError(t, p.Release("c"))
	must.FileContains(t, p.sharePath, "10-19")
	must.FileContains(t, p.reservePath, "")
}

func TestPartition_Release_overlapping(t *testing.T) {
	p := testPartition(t)

	// the scheduler can place a replacement alloc on the cores of the alloc
	// it replaces before the client has stopped the old one
	must.NoError(t, p.Reserve("old", coreset(10, 11, 12, 13)))
	must.NoError(t, p.Reserve("new", coreset(10, 11, 12, 13)))
	must.FileContains(t, p.sharePath, "14-19")
	must.FileContains(t, p.reservePath, "10-13")

	// stopping the old alloc must not take the cores away from the new one
	must.NoError(t, p.Release("old"))
	must.FileContains(t, p.sharePath, "14-19")
	must.FileContains(t, p.reservePath, "10-13")

	must.NoError(t, p.Release("new"))
	must.FileContains(t, p.sharePath, "10-19")
	must.FileContains(t, p.reservePath, "")
}

func TestPartition_Release_partialOverlap(t *testing.T) {
	p := testPartition(t)

	must.NoError(t, p.Reserve("a", coreset(10, 11, 12)))
	must.NoError(t, p.Reserve("b", coreset(12, 13, 14)))
	must.FileContains(t, p.sharePath, "15-19")
	must.FileContains(t, p.reservePath, "10-14")

	// only the cores held by no other alloc go back to the share
	must.NoError(t, p.Release("a"))
	must.FileContains(t, p.sharePath, "10-11,15-19")
	must.FileContains(t, p.reservePath, "12-14")
}

func TestPartition_Release_withoutReserve(t *testing.T) {
	p := testPartition(t)

	must.NoError(t, p.Reserve("a", coreset(10, 11)))

	// an earlier prerun hook failing means postrun runs without a matching
	// reserve; the cores of other allocs must be untouched
	must.NoError(t, p.Release("b"))
	must.FileContains(t, p.sharePath, "12-19")
	must.FileContains(t, p.reservePath, "10-11")
}

func TestPartition_write_error(t *testing.T) {
	p := testPartition(t)
	p.reservePath = filepath.Join(t.TempDir(), "missing", "cpuset.cpus")

	// errors other than ENOSPC on an empty set are returned
	err := p.Reserve("a", coreset(10))
	must.ErrorContains(t, err, "unable to update reserve cpuset")
	must.ErrorContains(t, err, p.reservePath)

	err = p.Release("a")
	must.ErrorContains(t, err, "unable to update reserve cpuset")
}
