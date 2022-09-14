package state

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/stretchr/testify/require"

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
	require.NoError(t, err)
	require.Nil(t, sve)
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

	var expectedQuotaSize int
	for _, v := range svs {
		expectedQuotaSize += len(v.Data)
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
			require.NoError(t, resp.Error)
		}

		// Check that the index for the table was modified as expected.
		initialIndex, err := testState.Index(TableVariables)
		require.NoError(t, err)
		require.Equal(t, insertIndex, initialIndex)

		// List all the variables in the table
		iter, err := testState.Variables(ws)
		require.NoError(t, err)

		got := []*structs.VariableEncrypted{}
		for raw := iter.Next(); raw != nil; raw = iter.Next() {
			sv := raw.(*structs.VariableEncrypted)
			var svCopy structs.VariableEncrypted
			svCopy = sv.Copy()
			got = append(got, &svCopy)
		}
		require.Len(t, got, 2, "incorrect number of variables found")

		// Ensure the create and modify indexes are populated correctly.
		require.Equal(t, uint64(21), got[0].CreateIndex, "%s: incorrect create index", got[0].Path)
		require.Equal(t, uint64(21), got[0].ModifyIndex, "%s: incorrect modify index", got[0].Path)
		require.Equal(t, uint64(22), got[1].CreateIndex, "%s: incorrect create index", got[1].Path)
		require.Equal(t, uint64(22), got[1].ModifyIndex, "%s: incorrect modify index", got[1].Path)

		quotaUsed, err := testState.VariablesQuotaByNamespace(ws, structs.DefaultNamespace)
		require.NoError(t, err)
		require.Equal(t, int64(expectedQuotaSize), quotaUsed.Size)

		// update the mocks so the test element has the correct create/modify
		// indexes and times now that we have validated them
		svs = got
	})

	t.Run("1a fetch variable", func(t *testing.T) {
		sve, err := testState.GetVariable(ws, svs[0].Namespace, svs[0].Path)
		require.NoError(t, err)
		require.NotNil(t, sve)
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
			require.NoError(t, resp.Error)
		}

		reInsertActualIndex, err := testState.Index(TableVariables)
		require.NoError(t, err)
		require.Equal(t, insertIndex, reInsertActualIndex, "index should not have changed")

		quotaUsed, err := testState.VariablesQuotaByNamespace(ws, structs.DefaultNamespace)
		require.NoError(t, err)
		require.Equal(t, int64(expectedQuotaSize), quotaUsed.Size)
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
		require.NoError(t, resp.Error)

		// Check that the index for the table was modified as expected.
		updateActualIndex, err := testState.Index(TableVariables)
		require.NoError(t, err)
		require.Equal(t, update1Index, updateActualIndex, "index should have changed")

		// Get the variables from the table.
		iter, err := testState.Variables(ws)
		require.NoError(t, err)

		got := []*structs.VariableEncrypted{}

		// Iterate all the stored variables and assert indexes have been updated as expected
		for raw := iter.Next(); raw != nil; raw = iter.Next() {
			sv := raw.(*structs.VariableEncrypted)
			var svCopy structs.VariableEncrypted
			svCopy = sv.Copy()
			got = append(got, &svCopy)
		}
		require.Len(t, got, 2)
		require.Equal(t, update1Index, got[0].ModifyIndex)
		require.Equal(t, insertIndex, got[1].ModifyIndex)

		// update the mocks so the test element has the correct create/modify
		// indexes and times now that we have validated them
		svs = got

		quotaUsed, err := testState.VariablesQuotaByNamespace(ws, structs.DefaultNamespace)
		require.NoError(t, err)
		require.Equal(t, int64(expectedQuotaSize+1), quotaUsed.Size)
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
		require.NoError(t, resp.Error)

		// Check that the index for the table was modified as expected.
		update2ActualIndex, err := testState.Index(TableVariables)
		require.NoError(t, err)
		require.Equal(t, update2Index, update2ActualIndex, "index should have changed")

		// Get the variables from the table.
		iter, err := testState.Variables(ws)
		require.NoError(t, err)

		got := []structs.VariableEncrypted{}

		// Iterate all the stored variables and assert indexes have been updated as expected
		for raw := iter.Next(); raw != nil; raw = iter.Next() {
			sv := raw.(*structs.VariableEncrypted)
			got = append(got, sv.Copy())
		}
		require.Len(t, got, 2)
		require.Equal(t, svs[0].ModifyIndex, got[0].ModifyIndex)
		require.Equal(t, update2Index, got[1].ModifyIndex)

		require.True(t, svs[0].Equals(got[0]))
		require.True(t, sv2.Equals(got[1]))

		quotaUsed, err := testState.VariablesQuotaByNamespace(ws, structs.DefaultNamespace)
		require.NoError(t, err)
		require.Equal(t, int64(expectedQuotaSize+1), quotaUsed.Size)

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
		require.NoError(t, resp.Error, "deleting non-existing var is not an error")

		actualInitialIndex, err := testState.Index(TableVariables)
		require.NoError(t, err)
		require.Equal(t, uint64(0), actualInitialIndex, "index should not have changed")

		quotaUsed, err := testState.VariablesQuotaByNamespace(nil, structs.DefaultNamespace)
		require.NoError(t, err)
		require.Nil(t, quotaUsed)
	})

	// Upsert two variables, deletes one, then ensure the
	// remaining is left as expected.
	t.Run("2 upsert variable and delete", func(t *testing.T) {

		ns := mock.Namespace()
		ns.Name = svs[0].Namespace
		require.NoError(t, testState.UpsertNamespaces(initialIndex, []*structs.Namespace{ns}))

		for _, sv := range svs {
			svReq := &structs.VarApplyStateRequest{
				Op:  structs.VarOpSet,
				Var: sv,
			}
			initialIndex++
			resp := testState.VarSet(initialIndex, svReq)
			require.NoError(t, resp.Error)
		}

		// Perform the delete.
		delete1Index := uint64(20)

		resp := testState.VarDelete(delete1Index, &structs.VarApplyStateRequest{
			Op:  structs.VarOpDelete,
			Var: svs[0],
		})
		require.NoError(t, resp.Error)

		// Check that the index for the table was modified as expected.
		actualDelete1Index, err := testState.Index(TableVariables)
		require.NoError(t, err)
		require.Equal(t, delete1Index, actualDelete1Index, "index should have changed")

		ws := memdb.NewWatchSet()

		// Get the variables from the table.
		iter, err := testState.Variables(ws)
		require.NoError(t, err)

		var delete1Count int
		var expectedQuotaSize int

		// Iterate all the stored variables and assert we have the expected
		// number.
		for raw := iter.Next(); raw != nil; raw = iter.Next() {
			delete1Count++
			v := raw.(*structs.VariableEncrypted)
			expectedQuotaSize += len(v.Data)
		}
		require.Equal(t, 1, delete1Count, "unexpected number of variables in table")
		quotaUsed, err := testState.VariablesQuotaByNamespace(ws, structs.DefaultNamespace)
		require.NoError(t, err)
		require.Equal(t, int64(expectedQuotaSize), quotaUsed.Size)
	})

	t.Run("3 delete remaining variable", func(t *testing.T) {
		delete2Index := uint64(30)

		resp := testState.VarDelete(delete2Index, &structs.VarApplyStateRequest{
			Op:  structs.VarOpDelete,
			Var: svs[1],
		})
		require.NoError(t, resp.Error)

		// Check that the index for the table was modified as expected.
		actualDelete2Index, err := testState.Index(TableVariables)
		require.NoError(t, err)
		require.Equal(t, delete2Index, actualDelete2Index, "index should have changed")

		// Get the variables from the table.
		ws := memdb.NewWatchSet()
		iter, err := testState.Variables(ws)
		require.NoError(t, err)

		var delete2Count int

		// Ensure the table is empty.
		for raw := iter.Next(); raw != nil; raw = iter.Next() {
			delete2Count++
		}
		require.Equal(t, 0, delete2Count, "unexpected number of variables in table")

		quotaUsed, err := testState.VariablesQuotaByNamespace(ws, structs.DefaultNamespace)
		require.NoError(t, err)
		require.Equal(t, int64(0), quotaUsed.Size)
	})
}

func TestStateStore_GetVariables(t *testing.T) {
	ci.Parallel(t)
	testState := testStateStore(t)

	ns := mock.Namespace()
	ns.Name = "~*magical*~"
	initialIndex := uint64(10)
	require.NoError(t, testState.UpsertNamespaces(initialIndex, []*structs.Namespace{ns}))

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
		require.NoError(t, resp.Error)
	}

	// Look up variables using the namespace of the first mock variable.
	ws := memdb.NewWatchSet()
	iter, err := testState.GetVariablesByNamespace(ws, svs[0].Namespace)
	require.NoError(t, err)

	var count1 int

	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		sv := raw.(*structs.VariableEncrypted)
		require.Equal(t, svs[0].Namespace, sv.Namespace)
		require.Equal(t, uint64(11), sv.CreateIndex, "%s incorrect create index", sv.Path)
		require.Equal(t, uint64(11), sv.ModifyIndex, "%s incorrect modify index", sv.Path)
		count1++
	}

	require.Equal(t, 1, count1)

	// Look up variables using the namespace of the second mock variable.
	iter, err = testState.GetVariablesByNamespace(ws, svs[1].Namespace)
	require.NoError(t, err)

	var count2 int

	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		count2++
		sv := raw.(*structs.VariableEncrypted)
		require.Equal(t, initialIndex, sv.CreateIndex, "%s incorrect create index", sv.Path)
		require.Equal(t, initialIndex, sv.ModifyIndex, "%s incorrect modify index", sv.Path)
		require.Equal(t, svs[1].Namespace, sv.Namespace)
	}
	require.Equal(t, 1, count2)

	// Look up variables using a namespace that shouldn't contain any
	// variables.
	iter, err = testState.GetVariablesByNamespace(ws, "pony-club")
	require.NoError(t, err)

	var count3 int

	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		count3++
	}
	require.Equal(t, 0, count3)
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
	require.NoError(t, testState.UpsertNamespaces(initialIndex, []*structs.Namespace{ns}))

	for _, sv := range svs {
		svReq := &structs.VarApplyStateRequest{
			Op:  structs.VarOpSet,
			Var: sv,
		}
		initialIndex++
		resp := testState.VarSet(initialIndex, svReq)
		require.NoError(t, resp.Error)
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
				require.NoError(t, err)

				var count int = 0
				for raw := iter.Next(); raw != nil; raw = iter.Next() {
					count++
					sv := raw.(*structs.VariableEncrypted)
					require.Equal(t, tC.namespace, sv.Namespace)
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
				require.NoError(t, err)

				var count int = 0
				for raw := iter.Next(); raw != nil; raw = iter.Next() {
					count++
					sv := raw.(*structs.VariableEncrypted)
					require.Equal(t, tC.namespace, sv.Namespace)
					require.True(t, strings.HasPrefix(sv.Path, tC.prefix))
				}
				require.Equal(t, tC.expectedCount, count)
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
				require.NoError(t, err)

				var count int = 0
				for raw := iter.Next(); raw != nil; raw = iter.Next() {
					count++
					sv := raw.(*structs.VariableEncrypted)
					require.True(t, strings.HasPrefix(sv.Path, tC.prefix))
				}
				require.Equal(t, tC.expectedCount, count)
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
		require.NoError(t, resp.Error)
	}

	ws := memdb.NewWatchSet()
	iter, err := testState.GetVariablesByKeyID(ws, keyID)
	require.NoError(t, err)

	var count int
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		sv := raw.(*structs.VariableEncrypted)
		require.Equal(t, keyID, sv.KeyID)
		require.Equal(t, expectedForKey[count], sv.Path)
		require.NotContains(t, expectedOrphaned, sv.Path)
		count++
	}
	require.Equal(t, 5, count)
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
		require.True(t, resp.IsOk())
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
		require.True(t, resp.IsConflict())
		require.NotNil(t, resp.Conflict)
		require.Equal(t, varZero.VariableMetadata, resp.Conflict.VariableMetadata)
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
		require.True(t, resp.IsOk(), "resp: %+v", resp)

		// A CAS delete with a zero index should return a conflict when the
		// variable exists in the state store. The conflict value should
		// be the current state of the variable at the path.
		req := &structs.VarApplyStateRequest{
			Op:  structs.VarOpDelete,
			Var: &svZero,
		}
		resp = ts.VarDeleteCAS(0, req)
		require.True(t, resp.IsConflict(), "resp: %+v", resp)
		require.NotNil(t, resp.Conflict)
		require.Equal(t, sv.VariableMetadata, resp.Conflict.VariableMetadata)
	})
	t.Run("real_var-cas_ok", func(t *testing.T) {
		ci.Parallel(t)
		sv := mock.VariableEncrypted()
		sv.Path = "real_var/cas_ok"
		resp := ts.VarSet(10, &structs.VarApplyStateRequest{
			Op:  structs.VarOpSet,
			Var: sv,
		})
		require.True(t, resp.IsOk())

		// A CAS delete with a correct index should succeed.
		req := &structs.VarApplyStateRequest{
			Op:  structs.VarOpDelete,
			Var: sv,
		}
		resp = ts.VarDeleteCAS(0, req)
		require.True(t, resp.IsOk())
	})
}
