// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package checkstore

import (
	"slices"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/serviceregistration/checks"
	"github.com/hashicorp/nomad/client/state"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

var (
	success = structs.CheckSuccess
	failure = structs.CheckFailure
	pending = structs.CheckPending
)

func newQR(id string, status structs.CheckStatus) *structs.CheckQueryResult {
	return &structs.CheckQueryResult{
		ID:     structs.CheckID(id),
		Status: status,
	}
}

// alias for brevity
type qrMap = map[structs.CheckID]*structs.CheckQueryResult

func TestShim_New(t *testing.T) {
	ci.Parallel(t)
	logger := testlog.HCLogger(t)

	t.Run("restore empty", func(t *testing.T) {
		db := state.NewMemDB(logger)
		s := NewStore(logger, db)
		m := s.List("none")
		must.MapEmpty(t, m)
	})

	t.Run("restore checks", func(t *testing.T) {
		db := state.NewMemDB(logger)
		must.NoError(t, db.PutCheckResult("alloc1", newQR("id1", success)))
		must.NoError(t, db.PutCheckResult("alloc1", newQR("id2", failure)))
		must.NoError(t, db.PutCheckResult("alloc2", newQR("id3", pending)))
		s := NewStore(logger, db)
		m1 := s.List("alloc1")
		must.MapEq(t, qrMap{
			"id1": newQR("id1", success),
			"id2": newQR("id2", failure),
		}, m1)
		m2 := s.List("alloc2")
		must.MapEq(t, qrMap{"id3": newQR("id3", pending)}, m2)
	})
}

func TestShim_Set(t *testing.T) {
	ci.Parallel(t)
	logger := testlog.HCLogger(t)

	t.Run("insert pending", func(t *testing.T) {
		db := state.NewMemDB(logger)
		s := NewStore(logger, db)

		// insert initial pending check into empty database
		qr1 := newQR("id1", pending)
		qr1.Timestamp = 1
		must.NoError(t, s.Set("alloc1", qr1))

		// ensure underlying db has check
		internal, err := db.GetCheckResults()
		must.NoError(t, err)
		must.Eq(t, checks.ClientResults{"alloc1": {"id1": qr1}}, internal)
	})

	t.Run("ignore followup pending", func(t *testing.T) {
		db := state.NewMemDB(logger)
		s := NewStore(logger, db)

		// insert a check
		qr1 := newQR("id1", success)
		qr1.Timestamp = 1
		must.NoError(t, s.Set("alloc1", qr1))

		// insert a followup pending check (e.g. client restart)
		qr2 := newQR("id1", pending)
		qr2.Timestamp = 2
		t.Run("into existing", func(t *testing.T) {
			must.NoError(t, s.Set("alloc1", qr2))
		})

		// ensure shim maintains success result
		list := s.List("alloc1")
		must.Eq(t, qrMap{"id1": qr1}, list)

		// ensure underlying db maintains success result
		internal, err := db.GetCheckResults()
		must.NoError(t, err)
		must.Eq(t, checks.ClientResults{"alloc1": {"id1": qr1}}, internal)
	})

	t.Run("insert status change", func(t *testing.T) {
		db := state.NewMemDB(logger)
		s := NewStore(logger, db)

		// insert initial check, success
		qr1 := newQR("id1", success)
		must.NoError(t, s.Set("alloc1", qr1))

		// insert followup check, failure
		qr2 := newQR("id1", failure)
		must.NoError(t, s.Set("alloc1", qr2))

		// ensure shim sees newest status result
		list := s.List("alloc1")
		must.Eq(t, qrMap{"id1": qr2}, list)

		// ensure underlying db sees newest status result
		internal, err := db.GetCheckResults()
		must.NoError(t, err)
		must.Eq(t, checks.ClientResults{"alloc1": {"id1": qr2}}, internal)
	})

	t.Run("insert status same", func(t *testing.T) {
		db := state.NewMemDB(logger)
		s := NewStore(logger, db)

		// insert initial check, success
		qr1 := newQR("id1", success)
		qr1.Timestamp = 1
		must.NoError(t, s.Set("alloc1", qr1))

		// insert followup check, also success
		qr2 := newQR("id1", success)
		qr2.Timestamp = 2
		must.NoError(t, s.Set("alloc1", qr2))

		// ensure shim sees newest status result
		list := s.List("alloc1")
		must.Eq(t, qrMap{"id1": qr2}, list)

		// ensure underlying db sees stale result (optimization)
		internal, err := db.GetCheckResults()
		must.NoError(t, err)
		must.Eq(t, checks.ClientResults{"alloc1": {"id1": qr1}}, internal)
	})
}

func TestShim_List(t *testing.T) {
	ci.Parallel(t)
	logger := testlog.HCLogger(t)

	t.Run("list empty", func(t *testing.T) {
		db := state.NewMemDB(logger)
		s := NewStore(logger, db)

		list := s.List("alloc1")
		must.MapEmpty(t, list)
	})

	t.Run("list mix", func(t *testing.T) {
		db := state.NewMemDB(logger)
		s := NewStore(logger, db)

		// insert some checks
		must.NoError(t, s.Set("alloc1", newQR("id1", success)))
		must.NoError(t, s.Set("alloc1", newQR("id2", failure)))
		must.NoError(t, s.Set("alloc2", newQR("id1", pending)))

		list1 := s.List("alloc1")
		must.MapEq(t, qrMap{
			"id1": newQR("id1", success),
			"id2": newQR("id2", failure),
		}, list1)

		list2 := s.List("alloc2")
		must.MapEq(t, qrMap{
			"id1": newQR("id1", pending),
		}, list2)

		internal, err := db.GetCheckResults()
		must.NoError(t, err)
		must.MapEq(t, checks.ClientResults{
			"alloc1": {
				"id1": newQR("id1", success),
				"id2": newQR("id2", failure),
			},
			"alloc2": {
				"id1": newQR("id1", pending),
			},
		}, internal)
	})
}

func TestShim_Difference(t *testing.T) {
	ci.Parallel(t)
	logger := testlog.HCLogger(t)

	t.Run("empty store", func(t *testing.T) {
		db := state.NewMemDB(logger)
		s := NewStore(logger, db)

		ids := []structs.CheckID{"id1", "id2", "id3"}
		unwanted := s.Difference("alloc1", ids)
		must.SliceEmpty(t, unwanted)
	})

	t.Run("empty unwanted", func(t *testing.T) {
		db := state.NewMemDB(logger)
		s := NewStore(logger, db)

		// insert some checks
		must.NoError(t, s.Set("alloc1", newQR("id1", success)))
		must.NoError(t, s.Set("alloc1", newQR("id2", failure)))
		must.NoError(t, s.Set("alloc2", newQR("id1", pending)))

		var ids []structs.CheckID
		var exp = []structs.CheckID{"id1", "id2"}
		unwanted := s.Difference("alloc1", ids)
		slices.Sort(unwanted)
		must.Eq(t, exp, unwanted)
	})

	t.Run("subset unwanted", func(t *testing.T) {
		db := state.NewMemDB(logger)
		s := NewStore(logger, db)

		// insert some checks
		must.NoError(t, s.Set("alloc1", newQR("id1", success)))
		must.NoError(t, s.Set("alloc1", newQR("id2", failure)))
		must.NoError(t, s.Set("alloc1", newQR("id3", success)))
		must.NoError(t, s.Set("alloc1", newQR("id4", success)))
		must.NoError(t, s.Set("alloc1", newQR("id5", pending)))

		ids := []structs.CheckID{"id1", "id3", "id4"}
		exp := []structs.CheckID{"id2", "id5"}
		unwanted := s.Difference("alloc1", ids)
		slices.Sort(unwanted)
		must.Eq(t, exp, unwanted)
	})

	t.Run("unexpected unwanted", func(t *testing.T) {
		db := state.NewMemDB(logger)
		s := NewStore(logger, db)

		// insert some checks
		must.NoError(t, s.Set("alloc1", newQR("id1", success)))
		must.NoError(t, s.Set("alloc1", newQR("id2", failure)))
		must.NoError(t, s.Set("alloc1", newQR("id3", success)))

		ids := []structs.CheckID{"id1", "id4"}
		exp := []structs.CheckID{"id2", "id3"}
		unwanted := s.Difference("alloc1", ids)
		slices.Sort(unwanted)
		must.Eq(t, exp, unwanted)
	})
}

func TestShim_Remove(t *testing.T) {
	ci.Parallel(t)
	logger := testlog.HCLogger(t)

	t.Run("remove from empty store", func(t *testing.T) {
		db := state.NewMemDB(logger)
		s := NewStore(logger, db)

		ids := []structs.CheckID{"id1", "id2"}
		err := s.Remove("alloc1", ids)
		must.NoError(t, err)
	})

	t.Run("remove empty set from store", func(t *testing.T) {
		db := state.NewMemDB(logger)
		s := NewStore(logger, db)

		// insert some checks
		must.NoError(t, s.Set("alloc1", newQR("id1", success)))
		must.NoError(t, s.Set("alloc1", newQR("id2", failure)))
		must.NoError(t, s.Set("alloc2", newQR("id1", pending)))

		var ids []structs.CheckID
		err := s.Remove("alloc1", ids)
		must.NoError(t, err)

		// ensure shim still contains checks
		list := s.List("alloc1")
		must.Eq(t, qrMap{"id1": newQR("id1", success), "id2": newQR("id2", failure)}, list)

		// ensure underlying db still contains all checks
		internal, err := db.GetCheckResults()
		must.NoError(t, err)
		must.Eq(t, checks.ClientResults{
			"alloc1": {
				"id1": newQR("id1", success),
				"id2": newQR("id2", failure),
			},
			"alloc2": {
				"id1": newQR("id1", pending),
			},
		}, internal)
	})

	t.Run("remove subset from store", func(t *testing.T) {
		db := state.NewMemDB(logger)
		s := NewStore(logger, db)

		// insert some checks
		must.NoError(t, s.Set("alloc1", newQR("id1", success)))
		must.NoError(t, s.Set("alloc1", newQR("id2", failure)))
		must.NoError(t, s.Set("alloc1", newQR("id3", success)))
		must.NoError(t, s.Set("alloc1", newQR("id4", pending)))
		must.NoError(t, s.Set("alloc2", newQR("id1", pending)))
		must.NoError(t, s.Set("alloc2", newQR("id2", success)))

		ids := []structs.CheckID{"id1", "id4"}
		err := s.Remove("alloc1", ids)
		must.NoError(t, err)

		// ensure shim still contains remaining checks
		list := s.List("alloc1")
		must.Eq(t, qrMap{"id2": newQR("id2", failure), "id3": newQR("id3", success)}, list)

		// ensure underlying db still contains remaining checks
		internal, err := db.GetCheckResults()
		must.NoError(t, err)
		must.MapEq(t, checks.ClientResults{
			"alloc1": {
				"id2": newQR("id2", failure),
				"id3": newQR("id3", success),
			},
			"alloc2": {
				"id1": newQR("id1", pending),
				"id2": newQR("id2", success),
			},
		}, internal)
	})
}

func TestShim_Purge(t *testing.T) {
	ci.Parallel(t)
	logger := testlog.HCLogger(t)

	t.Run("purge from empty", func(t *testing.T) {
		db := state.NewMemDB(logger)
		s := NewStore(logger, db)

		err := s.Purge("alloc1")
		must.NoError(t, err)
	})

	t.Run("purge one alloc", func(t *testing.T) {
		db := state.NewMemDB(logger)
		s := NewStore(logger, db)

		// insert some checks
		must.NoError(t, s.Set("alloc1", newQR("id1", success)))
		must.NoError(t, s.Set("alloc1", newQR("id2", failure)))
		must.NoError(t, s.Set("alloc2", newQR("id1", pending)))

		err := s.Purge("alloc1")
		must.NoError(t, err)

		// ensure alloc1 is gone from shim
		list1 := s.List("alloc1")
		must.MapEmpty(t, list1)

		// ensure alloc2 remains in shim
		list2 := s.List("alloc2")
		must.MapEq(t, qrMap{"id1": newQR("id1", pending)}, list2)

		// ensure alloc is gone from underlying db
		internal, err := db.GetCheckResults()
		must.NoError(t, err)
		must.MapEq(t, checks.ClientResults{
			"alloc2": {"id1": newQR("id1", pending)},
		}, internal)
	})
}

func TestShim_Snapshot(t *testing.T) {
	ci.Parallel(t)
	logger := testlog.HCLogger(t)

	db := state.NewMemDB(logger)
	s := NewStore(logger, db)

	id1, id2, id3 := uuid.Short(), uuid.Short(), uuid.Short()
	must.NoError(t, s.Set("allocation1", &structs.CheckQueryResult{
		ID:     structs.CheckID(id1),
		Status: "passing",
	}))
	must.NoError(t, s.Set("allocation1", &structs.CheckQueryResult{
		ID:     structs.CheckID(id2),
		Status: "failing",
	}))
	must.NoError(t, s.Set("allocation2", &structs.CheckQueryResult{
		ID:     structs.CheckID(id3),
		Status: "passing",
	}))

	snap := s.Snapshot()
	must.MapEq(t, map[string]string{
		id1: "passing",
		id2: "failing",
		id3: "passing",
	}, snap)
}
