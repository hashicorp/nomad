// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"testing"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/shoenig/test/must"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
)

func TestStateStore_GetVariable(t *testing.T) {
	ci.Parallel(t)
	testState := testStateStore(t)
	ws := memdb.NewWatchSet()
	sve, err := testState.GetVariable(ws, "default", "not/a/path")
	must.NoError(t, err)
	must.Nil(t, sve)
}

func TestStateStore_UpsertVariables(t *testing.T) {
	ci.Parallel(t)
	testState := testStateStore(t)
	ws := memdb.NewWatchSet()

	svs := []*structs.VariableEncrypted{
		mock.VariableEncrypted(),
		mock.VariableEncrypted(),
	}
	svs[0].Path = "aaaaa"
	svs[1].Path = "bbbbb"

	insertIndex := uint64(20)

	var expectedQuotaSize int64
	for _, v := range svs {
		expectedQuotaSize += int64(len(v.Data))
	}

	// Ensure new variables are inserted as expected with their
	// correct indexes, along with an update to the index table.
	t.Run("1 create new variables", func(t *testing.T) {
		// Perform the initial upsert of variables.
		for _, sv := range svs {
			insertIndex++
			resp := testState.VarSet(insertIndex, &structs.VarApplyStateRequest{
				Op:  structs.VarOpSet,
				Var: sv,
			})
			must.NoError(t, resp.Error)
		}

		// Check that the index for the table was modified as expected.
		initialIndex, err := testState.Index(TableVariables)
		must.NoError(t, err)
		must.Eq(t, insertIndex, initialIndex)

		// List all the variables in the table
		got, err := getAllVariables(testState, ws)
		must.NoError(t, err)
		must.Len(t, 2, got, must.Sprintf("incorrect number of variables found"))

		// Ensure the create and modify indexes are populated correctly.
		must.Eq(t, 21, got[0].CreateIndex, must.Sprintf("%s: incorrect create index", got[0].Path))
		must.Eq(t, 21, got[0].ModifyIndex, must.Sprintf("%s: incorrect modify index", got[0].Path))
		must.Eq(t, 22, got[1].CreateIndex, must.Sprintf("%s: incorrect create index", got[1].Path))
		must.Eq(t, 22, got[1].ModifyIndex, must.Sprintf("%s: incorrect modify index", got[1].Path))

		quotaUsed, err := testState.VariablesQuotaByNamespace(ws, structs.DefaultNamespace)
		must.NoError(t, err)
		must.Eq(t, expectedQuotaSize, quotaUsed.Size)

		// update the mocks so the test element has the correct create/modify
		// indexes and times now that we have validated them
		svs = got
	})

	t.Run("1a fetch variable", func(t *testing.T) {
		sve, err := testState.GetVariable(ws, svs[0].Namespace, svs[0].Path)
		must.NoError(t, err)
		must.NotNil(t, sve)
	})

	// Upsert the exact same variables without any modification. In this
	// case, the index table should not be updated, indicating no write actually
	// happened due to equality checking.
	t.Run("2 upsert same", func(t *testing.T) {
		reInsertIndex := uint64(30)

		for _, sv := range svs {
			svReq := &structs.VarApplyStateRequest{
				Op:  structs.VarOpSet,
				Var: sv,
			}
			reInsertIndex++
			resp := testState.VarSet(reInsertIndex, svReq)
			must.NoError(t, resp.Error)
		}

		reInsertActualIndex, err := testState.Index(TableVariables)
		must.NoError(t, err)
		must.Eq(t, insertIndex, reInsertActualIndex, must.Sprintf("index should not have changed"))

		quotaUsed, err := testState.VariablesQuotaByNamespace(ws, structs.DefaultNamespace)
		must.NoError(t, err)
		must.Eq(t, expectedQuotaSize, quotaUsed.Size)
	})

	// Modify a single one of the previously inserted variables and
	// performs an upsert. This ensures the index table is modified correctly
	// and that each variable is updated, or not, as expected.
	t.Run("3 modify one", func(t *testing.T) {
		sv1Update := svs[0].Copy()
		sv1Update.KeyID = "sv1-update"

		buf := make([]byte, 1+len(sv1Update.Data))
		copy(buf, sv1Update.Data)
		buf[len(buf)-1] = 'x'
		sv1Update.Data = buf

		update1Index := uint64(40)

		resp := testState.VarSet(update1Index, &structs.VarApplyStateRequest{
			Op:  structs.VarOpSet,
			Var: &sv1Update,
		})
		must.NoError(t, resp.Error)

		// Check that the index for the table was modified as expected.
		updateActualIndex, err := testState.Index(TableVariables)
		must.NoError(t, err)
		must.Eq(t, update1Index, updateActualIndex, must.Sprintf("index should have changed"))

		// Iterate all the stored variables and assert indexes have been updated as expected
		got, err := getAllVariables(testState, ws)
		must.NoError(t, err)
		must.Len(t, 2, got)
		must.Eq(t, update1Index, got[0].ModifyIndex)
		must.Eq(t, insertIndex, got[1].ModifyIndex)

		// update the mocks so the test element has the correct create/modify
		// indexes and times now that we have validated them
		svs = got

		quotaUsed, err := testState.VariablesQuotaByNamespace(ws, structs.DefaultNamespace)
		must.NoError(t, err)
		must.Eq(t, expectedQuotaSize+1, quotaUsed.Size)
	})

	// Modify the second variable but send an upsert request that
	// includes this and the already modified variable.
	t.Run("4 upsert other", func(t *testing.T) {
		update2Index := uint64(50)
		sv2 := svs[1].Copy()
		sv2.KeyID = "sv2-update"
		sv2.ModifyIndex = update2Index

		resp := testState.VarSet(update2Index, &structs.VarApplyStateRequest{
			Op:  structs.VarOpSet,
			Var: &sv2,
		})
		must.NoError(t, resp.Error)

		// Check that the index for the table was modified as expected.
		update2ActualIndex, err := testState.Index(TableVariables)
		must.NoError(t, err)
		must.Eq(t, update2Index, update2ActualIndex, must.Sprintf("index should have changed"))

		// Get the variables from the table.
		iter, err := testState.Variables(ws)
		must.NoError(t, err)

		got := []structs.VariableEncrypted{}

		// Iterate all the stored variables and assert indexes have been updated as expected
		for raw := iter.Next(); raw != nil; raw = iter.Next() {
			sv := raw.(*structs.VariableEncrypted)
			got = append(got, sv.Copy())
		}
		must.Len(t, 2, got)
		must.Eq(t, svs[0].ModifyIndex, got[0].ModifyIndex)
		must.Eq(t, update2Index, got[1].ModifyIndex)

		must.True(t, svs[0].Equal(got[0]))
		must.True(t, sv2.Equal(got[1]))

		quotaUsed, err := testState.VariablesQuotaByNamespace(ws, structs.DefaultNamespace)
		must.NoError(t, err)
		must.Eq(t, expectedQuotaSize+1, quotaUsed.Size)

	})

	// Acquire lock on first variable to test upserting on a locked variable.
	t.Run("5 lock and upsert", func(t *testing.T) {
		acquireIndex := uint64(60)
		sv3 := svs[0].Copy()
		sv3.VariableMetadata.Lock = &structs.VariableLock{
			ID: "theLockID",
		}

		resp := testState.VarLockAcquire(acquireIndex,
			&structs.VarApplyStateRequest{
				Op:  structs.VarOpLockAcquire,
				Var: &sv3,
			})

		must.NoError(t, resp.Error)

		// Check that the index for the table was modified as expected.
		afterAcquireIndex, err := testState.Index(TableVariables)
		must.NoError(t, err)
		must.Eq(t, acquireIndex, afterAcquireIndex)

		// Attempt to upsert variable without the lock ID
		update4Index := uint64(65)
		sv4 := svs[0].Copy()
		sv4.KeyID = "sv4-update"
		sv4.ModifyIndex = update4Index

		resp = testState.VarSet(update4Index, &structs.VarApplyStateRequest{
			Op:  structs.VarOpSet,
			Var: &sv4,
		})

		must.NoError(t, resp.Error)
		afterFailedUpsertIndex, err := testState.Index(TableVariables)
		must.NoError(t, err)

		must.Eq(t, afterAcquireIndex, afterFailedUpsertIndex, must.Sprintf("index should not have changed"))
		must.True(t, resp.IsConflict())

		// Attempt to upsert variable but this time include the lock ID
		sv4.VariableMetadata.Lock = &structs.VariableLock{
			ID: "theLockID",
		}

		resp = testState.VarSet(update4Index, &structs.VarApplyStateRequest{
			Op:  structs.VarOpSet,
			Var: &sv4,
		})
		must.NoError(t, resp.Error)

		// Check that the index for the table was modified as expected.
		updateActualIndex, err := testState.Index(TableVariables)
		must.NoError(t, err)
		must.Eq(t, update4Index, updateActualIndex, must.Sprintf("index should have changed"))

		// Iterate all the stored variables and assert indexes have been updated as expected
		got, err := getAllVariables(testState, ws)
		must.NoError(t, err)
		must.Len(t, 2, got)
		must.Eq(t, update4Index, got[0].ModifyIndex)
	})
}

func TestStateStore_DeleteVariable(t *testing.T) {
	ci.Parallel(t)
	testState := testStateStore(t)

	// Generate some test variables that we will use and modify throughout.
	svs := []*structs.VariableEncrypted{
		mock.VariableEncrypted(),
		mock.VariableEncrypted(),
	}
	svs[0].Path = "aaaaa"
	svs[1].Path = "bbbbb"

	initialIndex := uint64(10)

	t.Run("1 delete a variable that does not exist", func(t *testing.T) {

		resp := testState.VarDelete(initialIndex, &structs.VarApplyStateRequest{
			Op:  structs.VarOpDelete,
			Var: svs[0],
		})
		must.NoError(t, resp.Error, must.Sprintf("deleting non-existing var is not an error"))

		actualInitialIndex, err := testState.Index(TableVariables)
		must.NoError(t, err)
		must.Eq(t, 0, actualInitialIndex, must.Sprintf("index should not have changed"))

		quotaUsed, err := testState.VariablesQuotaByNamespace(nil, structs.DefaultNamespace)
		must.NoError(t, err)
		must.Nil(t, quotaUsed)
	})

	// Upsert two variables, deletes one, then ensure the
	// remaining is left as expected.
	t.Run("2 upsert variable and delete", func(t *testing.T) {

		ns := mock.Namespace()
		ns.Name = svs[0].Namespace
		must.NoError(t, testState.UpsertNamespaces(initialIndex, []*structs.Namespace{ns}))

		for _, sv := range svs {
			svReq := &structs.VarApplyStateRequest{
				Op:  structs.VarOpSet,
				Var: sv,
			}
			initialIndex++
			resp := testState.VarSet(initialIndex, svReq)
			must.NoError(t, resp.Error)
		}

		// Perform the delete.
		delete1Index := uint64(20)

		resp := testState.VarDelete(delete1Index, &structs.VarApplyStateRequest{
			Op:  structs.VarOpDelete,
			Var: svs[0],
		})
		must.NoError(t, resp.Error)

		// Check that the index for the table was modified as expected.
		actualDelete1Index, err := testState.Index(TableVariables)
		must.NoError(t, err)
		must.Eq(t, delete1Index, actualDelete1Index, must.Sprintf("index should have changed"))

		ws := memdb.NewWatchSet()

		// Get the variables from the table.
		iter, err := testState.Variables(ws)
		must.NoError(t, err)

		var delete1Count int
		var expectedQuotaSize int64

		// Iterate all the stored variables and assert we have the expected
		// number.
		for raw := iter.Next(); raw != nil; raw = iter.Next() {
			delete1Count++
			v := raw.(*structs.VariableEncrypted)
			expectedQuotaSize += int64(len(v.Data))
		}
		must.Eq(t, 1, delete1Count, must.Sprintf("unexpected number of variables in table"))
		quotaUsed, err := testState.VariablesQuotaByNamespace(ws, structs.DefaultNamespace)
		must.NoError(t, err)
		must.Eq(t, expectedQuotaSize, quotaUsed.Size)
	})

	t.Run("3 lock the variable and attempt to delete it", func(t *testing.T) {
		ws := memdb.NewWatchSet()
		acquireIndex := uint64(25)
		lsv := svs[1].Copy()
		lsv.VariableMetadata.Lock = &structs.VariableLock{
			ID: "theLockID",
		}

		resp := testState.VarLockAcquire(acquireIndex,
			&structs.VarApplyStateRequest{
				Op:  structs.VarOpLockAcquire,
				Var: &lsv,
			})

		must.NoError(t, resp.Error)
		must.True(t, resp.IsOk())

		deleteLockedIndex := uint64(27)

		// Attempt to delete without the lock ID
		resp2 := testState.VarDelete(deleteLockedIndex, &structs.VarApplyStateRequest{
			Op:  structs.VarOpDelete,
			Var: svs[1],
		})

		must.NoError(t, resp2.Error)
		must.True(t, resp2.IsConflict())

		// Check that the index for the table was  not modified.
		failedDeleteIndex, err := testState.Index(TableVariables)
		must.NoError(t, err)
		must.Eq(t, acquireIndex, failedDeleteIndex)

		svs, err := getAllVariables(testState, ws)
		must.NoError(t, err)
		must.One(t, len(svs))

		// Release lock
		releaseIndex := uint64(30)

		lsv.VariableMetadata.Lock = &structs.VariableLock{
			ID: "theLockID",
		}

		resp3 := testState.VarLockRelease(releaseIndex,
			&structs.VarApplyStateRequest{
				Op:  structs.VarOpLockRelease,
				Var: &lsv,
			})
		must.NoError(t, err)
		must.True(t, resp3.IsOk())

		svs, err = getAllVariables(testState, ws)
		must.NoError(t, err)
		must.One(t, len(svs))

	})

	t.Run("4 delete remaining variable", func(t *testing.T) {
		delete2Index := uint64(40)

		resp := testState.VarDelete(delete2Index, &structs.VarApplyStateRequest{
			Op:  structs.VarOpDelete,
			Var: svs[1],
		})
		must.NoError(t, resp.Error)

		// Check that the index for the table was modified as expected.
		actualDelete2Index, err := testState.Index(TableVariables)
		must.NoError(t, err)
		must.Eq(t, delete2Index, actualDelete2Index, must.Sprintf("index should have changed"))

		// Get the variables from the table.
		ws := memdb.NewWatchSet()
		iter, err := testState.Variables(ws)
		must.NoError(t, err)

		var delete2Count int

		// Ensure the table is empty.
		for raw := iter.Next(); raw != nil; raw = iter.Next() {
			delete2Count++
		}
		must.Eq(t, 0, delete2Count, must.Sprintf("unexpected number of variables in table"))

		quotaUsed, err := testState.VariablesQuotaByNamespace(ws, structs.DefaultNamespace)
		must.NoError(t, err)
		must.Eq(t, 0, quotaUsed.Size)
	})
}

func TestStateStore_GetVariables(t *testing.T) {
	ci.Parallel(t)
	testState := testStateStore(t)

	ns := mock.Namespace()
	ns.Name = "~*magical*~"
	initialIndex := uint64(10)
	must.NoError(t, testState.UpsertNamespaces(initialIndex, []*structs.Namespace{ns}))

	// Generate some test variables in different namespaces and upsert them.
	svs := []*structs.VariableEncrypted{
		mock.VariableEncrypted(),
		mock.VariableEncrypted(),
	}
	svs[0].Path = "aaaaa"
	svs[0].Namespace = "~*magical*~"
	svs[1].Path = "bbbbb"

	for _, sv := range svs {
		svReq := &structs.VarApplyStateRequest{
			Op:  structs.VarOpSet,
			Var: sv,
		}
		initialIndex++
		resp := testState.VarSet(initialIndex, svReq)
		must.NoError(t, resp.Error)
	}

	// Look up variables using the namespace of the first mock variable.
	ws := memdb.NewWatchSet()
	iter, err := testState.GetVariablesByNamespace(ws, svs[0].Namespace)
	must.NoError(t, err)

	var count1 int

	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		sv := raw.(*structs.VariableEncrypted)
		must.Eq(t, svs[0].Namespace, sv.Namespace)
		must.Eq(t, 11, sv.CreateIndex, must.Sprintf("%s incorrect create index", sv.Path))
		must.Eq(t, 11, sv.ModifyIndex, must.Sprintf("%s incorrect modify index", sv.Path))
		count1++
	}

	must.Eq(t, 1, count1)

	// Look up variables using the namespace of the second mock variable.
	iter, err = testState.GetVariablesByNamespace(ws, svs[1].Namespace)
	must.NoError(t, err)

	var count2 int

	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		count2++
		sv := raw.(*structs.VariableEncrypted)
		must.Eq(t, initialIndex, sv.CreateIndex, must.Sprintf("%s incorrect create index", sv.Path))
		must.Eq(t, initialIndex, sv.ModifyIndex, must.Sprintf("%s incorrect modify index", sv.Path))
		must.Eq(t, svs[1].Namespace, sv.Namespace)
	}
	must.Eq(t, 1, count2)

	// Look up variables using a namespace that shouldn't contain any
	// variables.
	iter, err = testState.GetVariablesByNamespace(ws, "pony-club")
	must.NoError(t, err)

	var count3 int

	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		count3++
	}
	must.Eq(t, 0, count3)
}

func TestStateStore_ListVariablesByNamespaceAndPrefix(t *testing.T) {
	ci.Parallel(t)
	testState := testStateStore(t)

	// Generate some test variables and upsert them.
	svs := []*structs.VariableEncrypted{}
	for i := 0; i < 6; i++ {
		sv := mock.VariableEncrypted()
		svs = append(svs, sv)
	}

	svs[0].Path = "a/b"
	svs[1].Path = "a/b/c"
	svs[2].Path = "unrelated/b/c"
	svs[3].Namespace = "other"
	svs[3].Path = "a/b/c"
	svs[4].Namespace = "other"
	svs[4].Path = "a/q/z"
	svs[5].Namespace = "other"
	svs[5].Path = "a/z/z"

	ns := mock.Namespace()
	ns.Name = "other"
	initialIndex := uint64(10)
	must.NoError(t, testState.UpsertNamespaces(initialIndex, []*structs.Namespace{ns}))

	for _, sv := range svs {
		svReq := &structs.VarApplyStateRequest{
			Op:  structs.VarOpSet,
			Var: sv,
		}
		initialIndex++
		resp := testState.VarSet(initialIndex, svReq)
		must.NoError(t, resp.Error)
	}

	t.Run("ByNamespace", func(t *testing.T) {
		testCases := []struct {
			desc          string
			namespace     string
			expectedCount int
		}{
			{
				desc:          "default",
				namespace:     "default",
				expectedCount: 2,
			},
			{
				desc:          "other",
				namespace:     "other",
				expectedCount: 3,
			},
			{
				desc:          "nonexistent",
				namespace:     "BAD",
				expectedCount: 0,
			},
		}

		ws := memdb.NewWatchSet()
		for _, tC := range testCases {
			t.Run(tC.desc, func(t *testing.T) {
				iter, err := testState.GetVariablesByNamespace(ws, tC.namespace)
				must.NoError(t, err)

				var count int = 0
				for raw := iter.Next(); raw != nil; raw = iter.Next() {
					count++
					sv := raw.(*structs.VariableEncrypted)
					must.Eq(t, tC.namespace, sv.Namespace)
				}
			})
		}
	})

	t.Run("ByNamespaceAndPrefix", func(t *testing.T) {
		testCases := []struct {
			desc          string
			namespace     string
			prefix        string
			expectedCount int
		}{
			{
				desc:          "ns1 with good path",
				namespace:     "default",
				prefix:        "a",
				expectedCount: 2,
			},
			{
				desc:          "ns2 with good path",
				namespace:     "other",
				prefix:        "a",
				expectedCount: 3,
			},
			{
				desc:          "ns1 path valid for ns2",
				namespace:     "default",
				prefix:        "a/b/c",
				expectedCount: 1,
			},
			{
				desc:          "ns2 empty prefix",
				namespace:     "other",
				prefix:        "",
				expectedCount: 3,
			},
			{
				desc:          "nonexistent ns",
				namespace:     "BAD",
				prefix:        "",
				expectedCount: 0,
			},
		}

		ws := memdb.NewWatchSet()
		for _, tC := range testCases {
			t.Run(tC.desc, func(t *testing.T) {
				iter, err := testState.GetVariablesByNamespaceAndPrefix(ws, tC.namespace, tC.prefix)
				must.NoError(t, err)

				var count int = 0
				for raw := iter.Next(); raw != nil; raw = iter.Next() {
					count++
					sv := raw.(*structs.VariableEncrypted)
					must.Eq(t, tC.namespace, sv.Namespace)
					must.True(t, strings.HasPrefix(sv.Path, tC.prefix))
				}
				must.Eq(t, tC.expectedCount, count)
			})
		}
	})

	t.Run("ByPrefix", func(t *testing.T) {
		testCases := []struct {
			desc          string
			prefix        string
			expectedCount int
		}{
			{
				desc:          "bad prefix",
				prefix:        "bad",
				expectedCount: 0,
			},
			{
				desc:          "multiple ns",
				prefix:        "a/b/c",
				expectedCount: 2,
			},
			{
				desc:          "all",
				prefix:        "",
				expectedCount: 6,
			},
		}

		ws := memdb.NewWatchSet()
		for _, tC := range testCases {
			t.Run(tC.desc, func(t *testing.T) {
				iter, err := testState.GetVariablesByPrefix(ws, tC.prefix)
				must.NoError(t, err)

				var count int = 0
				for raw := iter.Next(); raw != nil; raw = iter.Next() {
					count++
					sv := raw.(*structs.VariableEncrypted)
					must.True(t, strings.HasPrefix(sv.Path, tC.prefix))
				}
				must.Eq(t, tC.expectedCount, count)
			})
		}
	})
}

func TestStateStore_ListVariablesByKeyID(t *testing.T) {
	ci.Parallel(t)
	testState := testStateStore(t)

	// Generate some test variables and upsert them.
	svs := []*structs.VariableEncrypted{}
	for i := 0; i < 7; i++ {
		sv := mock.VariableEncrypted()
		sv.Path = uuid.Generate()
		svs = append(svs, sv)
	}

	keyID := uuid.Generate()

	expectedForKey := []string{}
	for i := 0; i < 5; i++ {
		svs[i].KeyID = keyID
		expectedForKey = append(expectedForKey, svs[i].Path)
		sort.Strings(expectedForKey)
	}

	expectedOrphaned := []string{svs[5].Path, svs[6].Path}

	initialIndex := uint64(10)

	for _, sv := range svs {
		svReq := &structs.VarApplyStateRequest{
			Op:  structs.VarOpSet,
			Var: sv,
		}
		initialIndex++
		resp := testState.VarSet(initialIndex, svReq)
		must.NoError(t, resp.Error)
	}

	ws := memdb.NewWatchSet()
	iter, err := testState.GetVariablesByKeyID(ws, keyID)
	must.NoError(t, err)

	var count int
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		sv := raw.(*structs.VariableEncrypted)
		must.Eq(t, keyID, sv.KeyID)
		must.Eq(t, expectedForKey[count], sv.Path)
		must.SliceNotContains(t, expectedOrphaned, sv.Path)
		count++
	}
	must.Eq(t, 5, count)
}

func printVariable(tsv *structs.VariableEncrypted) string {
	b, _ := json.Marshal(tsv)
	return string(b)
}

func printVariables(tsvs []*structs.VariableEncrypted) string {
	if len(tsvs) == 0 {
		return ""
	}
	var out strings.Builder
	for _, tsv := range tsvs {
		out.WriteString(printVariable(tsv) + "\n")
	}
	return out.String()
}

// TestStateStore_Variables_DeleteCAS
func TestStateStore_Variables_DeleteCAS(t *testing.T) {
	ci.Parallel(t)
	ts := testStateStore(t)

	varNotExist := structs.VariableEncrypted{
		VariableMetadata: structs.VariableMetadata{
			Namespace:   "default",
			Path:        "does/not/exist",
			ModifyIndex: 0,
		},
	}

	t.Run("missing_var-cas_0", func(t *testing.T) {
		ci.Parallel(t)
		varNotExist := varNotExist
		// A CAS delete with index 0 should succeed when the variable does not
		// exist in the state store.
		resp := ts.VarDeleteCAS(10, &structs.VarApplyStateRequest{
			Op:  structs.VarOpDelete,
			Var: &varNotExist,
		})
		must.True(t, resp.IsOk())
	})
	t.Run("missing_var-cas_1", func(t *testing.T) {
		ci.Parallel(t)
		varZero := varNotExist
		varNotExist := varNotExist
		// A CAS delete with a non-zero index should return a conflict when the
		// variable does not exist in the state store. The conflict value should
		// be a zero value having the same namespace and path.
		varNotExist.ModifyIndex = 1
		req := &structs.VarApplyStateRequest{
			Op:  structs.VarOpDelete,
			Var: &varNotExist,
		}
		resp := ts.VarDeleteCAS(10, req)
		must.True(t, resp.IsConflict())
		must.NotNil(t, resp.Conflict)
		must.Eq(t, varZero.VariableMetadata, resp.Conflict.VariableMetadata)
	})
	t.Run("real_var-cas_0", func(t *testing.T) {
		ci.Parallel(t)
		sv := mock.VariableEncrypted()
		sv.CreateIndex = 0
		sv.ModifyIndex = 0
		sv.Path = "real_var/cas_0"
		// Need to make a copy because VarSet mutates Var.
		svZero := sv.Copy()
		resp := ts.VarSet(10, &structs.VarApplyStateRequest{
			Op:  structs.VarOpSet,
			Var: sv,
		})
		must.True(t, resp.IsOk(), must.Sprintf("resp: %+v", resp))

		// A CAS delete with a zero index should return a conflict when the
		// variable exists in the state store. The conflict value should
		// be the current state of the variable at the path.
		req := &structs.VarApplyStateRequest{
			Op:  structs.VarOpDelete,
			Var: &svZero,
		}
		resp = ts.VarDeleteCAS(0, req)
		must.True(t, resp.IsConflict(), must.Sprintf("resp: %+v", resp))
		must.NotNil(t, resp.Conflict)
		must.Eq(t, sv.VariableMetadata, resp.Conflict.VariableMetadata)
	})

	t.Run("real_locked_var-cas_0", func(t *testing.T) {
		ci.Parallel(t)
		sv := mock.VariableEncrypted()
		sv.Path = "real_var/cas_0"
		resp := ts.VarSet(10, &structs.VarApplyStateRequest{
			Op:  structs.VarOpSet,
			Var: sv,
		})
		must.True(t, resp.IsOk())

		svCopy := sv.Copy()
		svCopy.VariableMetadata.Lock = &structs.VariableLock{
			ID: "theLockID",
		}

		resp = ts.VarLockAcquire(15,
			&structs.VarApplyStateRequest{
				Op:  structs.VarOpLockAcquire,
				Var: &svCopy,
			})

		must.True(t, resp.IsOk())

		// A CAS delete with a correct index should succeed.
		req := &structs.VarApplyStateRequest{
			Op:  structs.VarOpDelete,
			Var: sv,
		}

		resp = ts.VarDeleteCAS(15, req)
		must.True(t, resp.IsConflict())

		resp = ts.VarLockRelease(20,
			&structs.VarApplyStateRequest{
				Op:  structs.VarOpLockRelease,
				Var: &svCopy,
			})

		must.True(t, resp.IsOk())
	})

	t.Run("real_var-cas_ok", func(t *testing.T) {
		ci.Parallel(t)
		sv := mock.VariableEncrypted()
		sv.Path = "real_var/cas_ok"
		resp := ts.VarSet(10, &structs.VarApplyStateRequest{
			Op:  structs.VarOpSet,
			Var: sv,
		})
		must.True(t, resp.IsOk())

		// A CAS delete with a correct index should succeed.
		req := &structs.VarApplyStateRequest{
			Op:  structs.VarOpDelete,
			Var: sv,
		}
		resp = ts.VarDeleteCAS(10, req)
		must.True(t, resp.IsOk())
	})
}

func TestStateStore_AcquireAndReleaseLock(t *testing.T) {
	ci.Parallel(t)
	testState := testStateStore(t)
	ws := memdb.NewWatchSet()

	mv := mock.VariableEncrypted()

	mv.Path = "thePath"
	mv.Lock = &structs.VariableLock{
		ID: "theLockID",
	}

	insertIndex := uint64(20)

	allVars, err := getAllVariables(testState, ws)
	must.NoError(t, err)

	t.Run("1 lock on missing variable", func(t *testing.T) {
		/* Attempt to acquire the lock on a variable that doesn't exist. */
		resp := testState.VarLockAcquire(insertIndex,
			&structs.VarApplyStateRequest{
				Op:  structs.VarOpLockAcquire,
				Var: mv,
			})

		must.NoError(t, resp.Error)

		// Check that the index for the table was modified as expected.
		initialIndex, err := testState.Index(TableVariables)
		must.NoError(t, err)
		must.Eq(t, insertIndex, initialIndex)

		got, err := getAllVariables(testState, ws)
		must.Eq(t, len(allVars)+1, len(got), must.Sprintf("incorrect number of variables found"))

		// Ensure the create and modify indexes are populated correctly.
		must.Eq(t, 20, got[0].CreateIndex, must.Sprintf("%s: incorrect create index", got[0].Path))
		must.Eq(t, 20, got[0].ModifyIndex, must.Sprintf("%s: incorrect modify index", got[0].Path))

		// Ensure the lock was persisted.
		must.Eq(t, "theLockID", got[0].Lock.ID)
		allVars = got
	})

	t.Run("2 lock on same variable", func(t *testing.T) {
		/* Attempt to acquire the lock on the same variable again. */
		sv := *allVars[0]
		sv.Lock = &structs.VariableLock{
			ID: "aDifferentLockID",
		}

		resp := testState.VarLockAcquire(insertIndex+1,
			&structs.VarApplyStateRequest{
				Op:  structs.VarOpLockAcquire,
				Var: &sv,
			})

		must.NoError(t, resp.Error)
		must.Eq(t, structs.VarOpResultConflict, resp.Result)
		// Ensure the create and modify were NOT modified
		must.Eq(t, 20, sv.CreateIndex, must.Sprintf("%s: incorrect create index", sv.Path))
		must.Eq(t, 20, sv.ModifyIndex, must.Sprintf("%s: incorrect modify index", sv.Path))

	})
	t.Run("3 release lock", func(t *testing.T) {
		/*  Test to release the lock  */
		allVars, err := getAllVariables(testState, ws)
		releaseIndex := uint64(40)
		resp := testState.VarLockRelease(releaseIndex,
			&structs.VarApplyStateRequest{
				Op:  structs.VarOpLockRelease,
				Var: allVars[0],
			})

		must.NoError(t, resp.Error)

		// Check that the index for the table was modified as expected.
		afterReleaseIndex, err := testState.Index(TableVariables)

		must.NoError(t, err)
		must.Eq(t, releaseIndex, afterReleaseIndex)

		// Ensure the lock was removed, but the variable was not.
		sve, err := testState.GetVariable(ws, mv.Namespace, mv.Path)
		must.NoError(t, err)
		must.NotNil(t, sve)
		must.Nil(t, sve.VariableMetadata.Lock)

		// Ensure the create and modify indexes are populated correctly.
		must.Eq(t, 20, sve.CreateIndex, must.Sprintf("%s: incorrect create index", sve.Path))
		must.Eq(t, 40, sve.ModifyIndex, must.Sprintf("%s: incorrect modify index", sve.Path))

		// Ensure the variable data didn't change
		must.Eq(t, mv.Data, sve.Data)
	})

	t.Run("3 reacquire lock", func(t *testing.T) {
		/*  Reacquire the lock, testing the mechanism to lock a previously existing variable */
		acquireIndex := uint64(60)
		resp := testState.VarLockAcquire(acquireIndex,
			&structs.VarApplyStateRequest{
				Op:  structs.VarOpLockAcquire,
				Var: mv,
			})

		must.NoError(t, resp.Error)

		// Check that the index for the table was modified as expected.
		afterAcquireIndex, err := testState.Index(TableVariables)
		must.NoError(t, err)
		must.Eq(t, acquireIndex, afterAcquireIndex)

		sve, err := testState.GetVariable(ws, mv.Namespace, mv.Path)
		must.NoError(t, err)

		// Ensure the create and modify indexes are populated correctly.
		must.Eq(t, 20, sve.CreateIndex, must.Sprintf("%s: incorrect create index", sve.Path))
		must.Eq(t, 60, sve.ModifyIndex, must.Sprintf("%s: incorrect modify index", sve.Path))

		// Ensure the lock was persisted again.
		must.Eq(t, "theLockID", sve.Lock.ID)

		// Ensure the variable data didn't change
		must.Eq(t, mv.Data, sve.Data)
	})
}

func TestStateStore_ReleaseLock(t *testing.T) {
	ci.Parallel(t)
	testState := testStateStore(t)

	insertIndex := uint64(20)
	resp := testState.VarSet(insertIndex, &structs.VarApplyStateRequest{
		Op: structs.VarOpSet,
		Var: &structs.VariableEncrypted{
			VariableMetadata: structs.VariableMetadata{
				Path:      "/non/lock/variable/path",
				Namespace: "default",
			},
			VariableData: mock.VariableEncrypted().VariableData,
		},
	})
	insertIndex++
	must.NoError(t, resp.Error)

	resp = testState.VarSet(insertIndex, &structs.VarApplyStateRequest{
		Op: structs.VarOpSet,
		Var: &structs.VariableEncrypted{
			VariableMetadata: structs.VariableMetadata{
				Path:      "lock/variable/path",
				Namespace: "default",
				Lock: &structs.VariableLock{
					ID: "theLockID",
				},
			},
			VariableData: mock.VariableEncrypted().VariableData,
		},
	})
	must.NoError(t, resp.Error)

	testCases := []struct {
		name       string
		lookUpPath string
		lockID     string
		expErr     error
		expResult  structs.VarOpResult
	}{
		{
			name:       "variable_not_found",
			lookUpPath: "fake/path/",
			expErr:     errVarNotFound,
			expResult:  structs.VarOpResultError,
		},
		{
			name:       "variable_has_no_lock",
			lookUpPath: "/non/lock/variable/path",
			expErr:     errLockNotFound,
			expResult:  structs.VarOpResultError,
		},
		{
			name:       "lock_id_doesn't_match",
			lookUpPath: "lock/variable/path",
			lockID:     "wrongLockID",
			expErr:     nil,
			expResult:  structs.VarOpResultConflict,
		},
		{
			name:       "lock_released",
			lookUpPath: "lock/variable/path",
			lockID:     "theLockID",
			expErr:     nil,
			expResult:  structs.VarOpResultOk,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			req := &structs.VarApplyStateRequest{
				Op: structs.VarOpLockRelease,
				Var: &structs.VariableEncrypted{
					VariableMetadata: structs.VariableMetadata{
						Path:      tc.lookUpPath,
						Namespace: "default",
					},
				},
			}

			if tc.lockID != "" {
				req.Var.VariableMetadata.Lock = &structs.VariableLock{
					ID: tc.lockID,
				}
			}

			resp = testState.VarLockRelease(insertIndex, req)

			if !errors.Is(tc.expErr, resp.Error) {
				t.Fatalf("expected error, got %s", resp.Error)
			}

			must.Eq(t, tc.expResult, resp.Result)
		})
	}
}

func TestStateStore_Release(t *testing.T) {
	ci.Parallel(t)
	testState := testStateStore(t)

	insertIndex := uint64(20)
	resp := testState.VarSet(insertIndex, &structs.VarApplyStateRequest{
		Op: structs.VarOpSet,
		Var: &structs.VariableEncrypted{
			VariableMetadata: structs.VariableMetadata{
				Path:      "/non/lock/variable/path",
				Namespace: "default",
			},
			VariableData: mock.VariableEncrypted().VariableData,
		},
	})
	insertIndex++
	must.NoError(t, resp.Error)

	resp = testState.VarSet(insertIndex, &structs.VarApplyStateRequest{
		Op: structs.VarOpSet,
		Var: &structs.VariableEncrypted{
			VariableMetadata: structs.VariableMetadata{
				Path:      "lock/variable/path",
				Namespace: "default",
				Lock: &structs.VariableLock{
					ID: "theLockID",
				},
			},
			VariableData: mock.VariableEncrypted().VariableData,
		},
	})
	must.NoError(t, resp.Error)

	testCases := []struct {
		name       string
		lookUpPath string
		lockID     string
		expErr     error
		expResult  structs.VarOpResult
	}{
		{
			name:       "variable_not_found",
			lookUpPath: "fake/path/",
			expErr:     errVarNotFound,
			expResult:  structs.VarOpResultError,
		},
		{
			name:       "variable_has_no_lock",
			lookUpPath: "/non/lock/variable/path",
			expErr:     errLockNotFound,
			expResult:  structs.VarOpResultError,
		},
		{
			name:       "lock_id_doesn't_match",
			lookUpPath: "lock/variable/path",
			lockID:     "wrongLockID",
			expErr:     nil,
			expResult:  structs.VarOpResultConflict,
		},
		{
			name:       "lock_released",
			lookUpPath: "lock/variable/path",
			lockID:     "theLockID",
			expErr:     nil,
			expResult:  structs.VarOpResultOk,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			req := &structs.VarApplyStateRequest{
				Op: structs.VarOpLockRelease,
				Var: &structs.VariableEncrypted{
					VariableMetadata: structs.VariableMetadata{
						Path:      tc.lookUpPath,
						Namespace: "default",
					},
				},
			}

			if tc.lockID != "" {
				req.Var.VariableMetadata.Lock = &structs.VariableLock{
					ID: tc.lockID,
				}
			}

			resp = testState.VarLockRelease(insertIndex, req)

			if !errors.Is(tc.expErr, resp.Error) {
				t.Fatalf("expected error, got %s", resp.Error)
			}

			must.Eq(t, tc.expResult, resp.Result)
		})
	}
}

func getAllVariables(ss *StateStore, ws memdb.WatchSet) ([]*structs.VariableEncrypted, error) {
	// List all the variables in the table
	iter, err := ss.Variables(ws)
	if err != nil {
		return []*structs.VariableEncrypted{}, err
	}

	got := []*structs.VariableEncrypted{}
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		sv := raw.(*structs.VariableEncrypted)
		var svCopy structs.VariableEncrypted
		svCopy = sv.Copy()
		got = append(got, &svCopy)
	}

	return got, nil
}
