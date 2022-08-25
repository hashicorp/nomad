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

func TestStateStore_GetSecureVariable(t *testing.T) {
	ci.Parallel(t)
	testState := testStateStore(t)
	ws := memdb.NewWatchSet()
	sve, err := testState.GetSecureVariable(ws, "default", "not/a/path")
	require.NoError(t, err)
	require.Nil(t, sve)
}

func TestStateStore_UpsertSecureVariables(t *testing.T) {
	ci.Parallel(t)
	testState := testStateStore(t)
	ws := memdb.NewWatchSet()

	svs := []*structs.SecureVariableEncrypted{
		mock.SecureVariableEncrypted(),
		mock.SecureVariableEncrypted(),
	}
	svs[0].Path = "aaaaa"
	svs[1].Path = "bbbbb"

	insertIndex := uint64(20)

	var expectedQuotaSize int
	for _, v := range svs {
		expectedQuotaSize += len(v.Data)
	}

	// Ensure new secure variables are inserted as expected with their
	// correct indexes, along with an update to the index table.
	t.Run("1 create new variables", func(t *testing.T) {
		// Perform the initial upsert of secure variables.
		for _, sv := range svs {
			insertIndex++
			resp := testState.SVESet(insertIndex, &structs.SVApplyStateRequest{
				Op:  structs.SVOpSet,
				Var: sv,
			})
			require.NoError(t, resp.Error)
		}

		// Check that the index for the table was modified as expected.
		initialIndex, err := testState.Index(TableSecureVariables)
		require.NoError(t, err)
		require.Equal(t, insertIndex, initialIndex)

		// List all the secure variables in the table
		iter, err := testState.SecureVariables(ws)
		require.NoError(t, err)

		got := []*structs.SecureVariableEncrypted{}
		for raw := iter.Next(); raw != nil; raw = iter.Next() {
			sv := raw.(*structs.SecureVariableEncrypted)
			var svCopy structs.SecureVariableEncrypted
			svCopy = sv.Copy()
			got = append(got, &svCopy)
		}
		require.Len(t, got, 2, "incorrect number of secure variables found")

		// Ensure the create and modify indexes are populated correctly.
		require.Equal(t, uint64(21), got[0].CreateIndex, "%s: incorrect create index", got[0].Path)
		require.Equal(t, uint64(21), got[0].ModifyIndex, "%s: incorrect modify index", got[0].Path)
		require.Equal(t, uint64(22), got[1].CreateIndex, "%s: incorrect create index", got[1].Path)
		require.Equal(t, uint64(22), got[1].ModifyIndex, "%s: incorrect modify index", got[1].Path)

		quotaUsed, err := testState.SecureVariablesQuotaByNamespace(ws, structs.DefaultNamespace)
		require.NoError(t, err)
		require.Equal(t, int64(expectedQuotaSize), quotaUsed.Size)

		// update the mocks so the test element has the correct create/modify
		// indexes and times now that we have validated them
		svs = got
	})

	t.Run("1a fetch variable", func(t *testing.T) {
		sve, err := testState.GetSecureVariable(ws, svs[0].Namespace, svs[0].Path)
		require.NoError(t, err)
		require.NotNil(t, sve)
	})

	// Upsert the exact same secure variables without any modification. In this
	// case, the index table should not be updated, indicating no write actually
	// happened due to equality checking.
	t.Run("2 upsert same", func(t *testing.T) {
		reInsertIndex := uint64(30)

		for _, sv := range svs {
			svReq := &structs.SVApplyStateRequest{
				Op:  structs.SVOpSet,
				Var: sv,
			}
			reInsertIndex++
			resp := testState.SVESet(reInsertIndex, svReq)
			require.NoError(t, resp.Error)
		}

		reInsertActualIndex, err := testState.Index(TableSecureVariables)
		require.NoError(t, err)
		require.Equal(t, insertIndex, reInsertActualIndex, "index should not have changed")

		quotaUsed, err := testState.SecureVariablesQuotaByNamespace(ws, structs.DefaultNamespace)
		require.NoError(t, err)
		require.Equal(t, int64(expectedQuotaSize), quotaUsed.Size)
	})

	// Modify a single one of the previously inserted secure variables and
	// performs an upsert. This ensures the index table is modified correctly
	// and that each secure variable is updated, or not, as expected.
	t.Run("3 modify one", func(t *testing.T) {
		sv1Update := svs[0].Copy()
		sv1Update.KeyID = "sv1-update"

		buf := make([]byte, 1+len(sv1Update.Data))
		copy(buf, sv1Update.Data)
		buf[len(buf)-1] = 'x'
		sv1Update.Data = buf

		update1Index := uint64(40)

		resp := testState.SVESet(update1Index, &structs.SVApplyStateRequest{
			Op:  structs.SVOpSet,
			Var: &sv1Update,
		})
		require.NoError(t, resp.Error)

		// Check that the index for the table was modified as expected.
		updateActualIndex, err := testState.Index(TableSecureVariables)
		require.NoError(t, err)
		require.Equal(t, update1Index, updateActualIndex, "index should have changed")

		// Get the secure variables from the table.
		iter, err := testState.SecureVariables(ws)
		require.NoError(t, err)

		got := []*structs.SecureVariableEncrypted{}

		// Iterate all the stored variables and assert indexes have been updated as expected
		for raw := iter.Next(); raw != nil; raw = iter.Next() {
			sv := raw.(*structs.SecureVariableEncrypted)
			var svCopy structs.SecureVariableEncrypted
			svCopy = sv.Copy()
			got = append(got, &svCopy)
		}
		require.Len(t, got, 2)
		require.Equal(t, update1Index, got[0].ModifyIndex)
		require.Equal(t, insertIndex, got[1].ModifyIndex)

		// update the mocks so the test element has the correct create/modify
		// indexes and times now that we have validated them
		svs = got

		quotaUsed, err := testState.SecureVariablesQuotaByNamespace(ws, structs.DefaultNamespace)
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

		resp := testState.SVESet(update2Index, &structs.SVApplyStateRequest{
			Op:  structs.SVOpSet,
			Var: &sv2,
		})
		require.NoError(t, resp.Error)

		// Check that the index for the table was modified as expected.
		update2ActualIndex, err := testState.Index(TableSecureVariables)
		require.NoError(t, err)
		require.Equal(t, update2Index, update2ActualIndex, "index should have changed")

		// Get the secure variables from the table.
		iter, err := testState.SecureVariables(ws)
		require.NoError(t, err)

		got := []structs.SecureVariableEncrypted{}

		// Iterate all the stored variables and assert indexes have been updated as expected
		for raw := iter.Next(); raw != nil; raw = iter.Next() {
			sv := raw.(*structs.SecureVariableEncrypted)
			got = append(got, sv.Copy())
		}
		require.Len(t, got, 2)
		require.Equal(t, svs[0].ModifyIndex, got[0].ModifyIndex)
		require.Equal(t, update2Index, got[1].ModifyIndex)

		require.True(t, svs[0].Equals(got[0]))
		require.True(t, sv2.Equals(got[1]))

		quotaUsed, err := testState.SecureVariablesQuotaByNamespace(ws, structs.DefaultNamespace)
		require.NoError(t, err)
		require.Equal(t, int64(expectedQuotaSize+1), quotaUsed.Size)

	})
}

func TestStateStore_DeleteSecureVariable(t *testing.T) {
	ci.Parallel(t)
	testState := testStateStore(t)

	// Generate some test secure variables that we will use and modify throughout.
	svs := []*structs.SecureVariableEncrypted{
		mock.SecureVariableEncrypted(),
		mock.SecureVariableEncrypted(),
	}
	svs[0].Path = "aaaaa"
	svs[1].Path = "bbbbb"

	initialIndex := uint64(10)

	t.Run("1 delete a secure variable that does not exist", func(t *testing.T) {

		resp := testState.SVEDelete(initialIndex, &structs.SVApplyStateRequest{
			Op:  structs.SVOpDelete,
			Var: svs[0],
		})
		require.NoError(t, resp.Error, "deleting non-existing secure var is not an error")

		actualInitialIndex, err := testState.Index(TableSecureVariables)
		require.NoError(t, err)
		require.Equal(t, uint64(0), actualInitialIndex, "index should not have changed")

		quotaUsed, err := testState.SecureVariablesQuotaByNamespace(nil, structs.DefaultNamespace)
		require.NoError(t, err)
		require.Nil(t, quotaUsed)
	})

	// Upsert two secure variables, deletes one, then ensure the
	// remaining is left as expected.
	t.Run("2 upsert variable and delete", func(t *testing.T) {

		ns := mock.Namespace()
		ns.Name = svs[0].Namespace
		require.NoError(t, testState.UpsertNamespaces(initialIndex, []*structs.Namespace{ns}))

		for _, sv := range svs {
			svReq := &structs.SVApplyStateRequest{
				Op:  structs.SVOpSet,
				Var: sv,
			}
			initialIndex++
			resp := testState.SVESet(initialIndex, svReq)
			require.NoError(t, resp.Error)
		}

		// Perform the delete.
		delete1Index := uint64(20)

		resp := testState.SVEDelete(delete1Index, &structs.SVApplyStateRequest{
			Op:  structs.SVOpDelete,
			Var: svs[0],
		})
		require.NoError(t, resp.Error)

		// Check that the index for the table was modified as expected.
		actualDelete1Index, err := testState.Index(TableSecureVariables)
		require.NoError(t, err)
		require.Equal(t, delete1Index, actualDelete1Index, "index should have changed")

		ws := memdb.NewWatchSet()

		// Get the secure variables from the table.
		iter, err := testState.SecureVariables(ws)
		require.NoError(t, err)

		var delete1Count int
		var expectedQuotaSize int

		// Iterate all the stored variables and assert we have the expected
		// number.
		for raw := iter.Next(); raw != nil; raw = iter.Next() {
			delete1Count++
			v := raw.(*structs.SecureVariableEncrypted)
			expectedQuotaSize += len(v.Data)
		}
		require.Equal(t, 1, delete1Count, "unexpected number of variables in table")
		quotaUsed, err := testState.SecureVariablesQuotaByNamespace(ws, structs.DefaultNamespace)
		require.NoError(t, err)
		require.Equal(t, int64(expectedQuotaSize), quotaUsed.Size)
	})

	t.Run("3 delete remaining variable", func(t *testing.T) {
		delete2Index := uint64(30)

		resp := testState.SVEDelete(delete2Index, &structs.SVApplyStateRequest{
			Op:  structs.SVOpDelete,
			Var: svs[1],
		})
		require.NoError(t, resp.Error)

		// Check that the index for the table was modified as expected.
		actualDelete2Index, err := testState.Index(TableSecureVariables)
		require.NoError(t, err)
		require.Equal(t, delete2Index, actualDelete2Index, "index should have changed")

		// Get the secure variables from the table.
		ws := memdb.NewWatchSet()
		iter, err := testState.SecureVariables(ws)
		require.NoError(t, err)

		var delete2Count int

		// Ensure the table is empty.
		for raw := iter.Next(); raw != nil; raw = iter.Next() {
			delete2Count++
		}
		require.Equal(t, 0, delete2Count, "unexpected number of variables in table")

		quotaUsed, err := testState.SecureVariablesQuotaByNamespace(ws, structs.DefaultNamespace)
		require.NoError(t, err)
		require.Equal(t, int64(0), quotaUsed.Size)
	})
}

func TestStateStore_GetSecureVariables(t *testing.T) {
	ci.Parallel(t)
	testState := testStateStore(t)

	ns := mock.Namespace()
	ns.Name = "~*magical*~"
	initialIndex := uint64(10)
	require.NoError(t, testState.UpsertNamespaces(initialIndex, []*structs.Namespace{ns}))

	// Generate some test secure variables in different namespaces and upsert them.
	svs := []*structs.SecureVariableEncrypted{
		mock.SecureVariableEncrypted(),
		mock.SecureVariableEncrypted(),
	}
	svs[0].Path = "aaaaa"
	svs[0].Namespace = "~*magical*~"
	svs[1].Path = "bbbbb"

	for _, sv := range svs {
		svReq := &structs.SVApplyStateRequest{
			Op:  structs.SVOpSet,
			Var: sv,
		}
		initialIndex++
		resp := testState.SVESet(initialIndex, svReq)
		require.NoError(t, resp.Error)
	}

	// Look up secure variables using the namespace of the first mock variable.
	ws := memdb.NewWatchSet()
	iter, err := testState.GetSecureVariablesByNamespace(ws, svs[0].Namespace)
	require.NoError(t, err)

	var count1 int

	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		sv := raw.(*structs.SecureVariableEncrypted)
		require.Equal(t, svs[0].Namespace, sv.Namespace)
		require.Equal(t, uint64(11), sv.CreateIndex, "%s incorrect create index", sv.Path)
		require.Equal(t, uint64(11), sv.ModifyIndex, "%s incorrect modify index", sv.Path)
		count1++
	}

	require.Equal(t, 1, count1)

	// Look up variables using the namespace of the second mock variable.
	iter, err = testState.GetSecureVariablesByNamespace(ws, svs[1].Namespace)
	require.NoError(t, err)

	var count2 int

	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		count2++
		sv := raw.(*structs.SecureVariableEncrypted)
		require.Equal(t, initialIndex, sv.CreateIndex, "%s incorrect create index", sv.Path)
		require.Equal(t, initialIndex, sv.ModifyIndex, "%s incorrect modify index", sv.Path)
		require.Equal(t, svs[1].Namespace, sv.Namespace)
	}
	require.Equal(t, 1, count2)

	// Look up variables using a namespace that shouldn't contain any
	// variables.
	iter, err = testState.GetSecureVariablesByNamespace(ws, "pony-club")
	require.NoError(t, err)

	var count3 int

	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		count3++
	}
	require.Equal(t, 0, count3)
}

func TestStateStore_ListSecureVariablesByNamespaceAndPrefix(t *testing.T) {
	ci.Parallel(t)
	testState := testStateStore(t)

	// Generate some test secure variables and upsert them.
	svs := []*structs.SecureVariableEncrypted{}
	for i := 0; i < 6; i++ {
		sv := mock.SecureVariableEncrypted()
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
		svReq := &structs.SVApplyStateRequest{
			Op:  structs.SVOpSet,
			Var: sv,
		}
		initialIndex++
		resp := testState.SVESet(initialIndex, svReq)
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
				iter, err := testState.GetSecureVariablesByNamespace(ws, tC.namespace)
				require.NoError(t, err)

				var count int = 0
				for raw := iter.Next(); raw != nil; raw = iter.Next() {
					count++
					sv := raw.(*structs.SecureVariableEncrypted)
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
				iter, err := testState.GetSecureVariablesByNamespaceAndPrefix(ws, tC.namespace, tC.prefix)
				require.NoError(t, err)

				var count int = 0
				for raw := iter.Next(); raw != nil; raw = iter.Next() {
					count++
					sv := raw.(*structs.SecureVariableEncrypted)
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
				iter, err := testState.GetSecureVariablesByPrefix(ws, tC.prefix)
				require.NoError(t, err)

				var count int = 0
				for raw := iter.Next(); raw != nil; raw = iter.Next() {
					count++
					sv := raw.(*structs.SecureVariableEncrypted)
					require.True(t, strings.HasPrefix(sv.Path, tC.prefix))
				}
				require.Equal(t, tC.expectedCount, count)
			})
		}
	})
}
func TestStateStore_ListSecureVariablesByKeyID(t *testing.T) {
	ci.Parallel(t)
	testState := testStateStore(t)

	// Generate some test secure variables and upsert them.
	svs := []*structs.SecureVariableEncrypted{}
	for i := 0; i < 7; i++ {
		sv := mock.SecureVariableEncrypted()
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
		svReq := &structs.SVApplyStateRequest{
			Op:  structs.SVOpSet,
			Var: sv,
		}
		initialIndex++
		resp := testState.SVESet(initialIndex, svReq)
		require.NoError(t, resp.Error)
	}

	ws := memdb.NewWatchSet()
	iter, err := testState.GetSecureVariablesByKeyID(ws, keyID)
	require.NoError(t, err)

	var count int
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		sv := raw.(*structs.SecureVariableEncrypted)
		require.Equal(t, keyID, sv.KeyID)
		require.Equal(t, expectedForKey[count], sv.Path)
		require.NotContains(t, expectedOrphaned, sv.Path)
		count++
	}
	require.Equal(t, 5, count)
}

func printSecureVariable(tsv *structs.SecureVariableEncrypted) string {
	b, _ := json.Marshal(tsv)
	return string(b)
}

func printSecureVariables(tsvs []*structs.SecureVariableEncrypted) string {
	if len(tsvs) == 0 {
		return ""
	}
	var out strings.Builder
	for _, tsv := range tsvs {
		out.WriteString(printSecureVariable(tsv) + "\n")
	}
	return out.String()
}
