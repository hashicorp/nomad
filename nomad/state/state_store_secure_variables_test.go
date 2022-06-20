package state

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"testing"
	"time"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
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

	svs, svm := mockSecureVariables(2, 2)
	t.Log(printSecureVariables(svs))
	insertIndex := uint64(20)
	t.Run("1 create new varibles", func(t *testing.T) {
		// SubTest Marker: This ensures new secure variables are inserted as
		// expected with their correct indexes, along with an update to the index
		// table.

		// Perform the initial upsert of secure variables.
		err := testState.UpsertSecureVariables(structs.MsgTypeTestSetup, insertIndex, svs)
		require.NoError(t, err)

		// Check that the index for the table was modified as expected.
		initialIndex, err := testState.Index(TableSecureVariables)
		require.NoError(t, err)
		require.Equal(t, insertIndex, initialIndex)

		// List all the secure variables in the table, so we can perform a
		// number of tests on the return array.

		iter, err := testState.SecureVariables(ws)
		require.NoError(t, err)

		// Count how many table entries we have, to ensure it is the expected
		// number.
		var count int

		for raw := iter.Next(); raw != nil; raw = iter.Next() {
			count++

			// Ensure the create and modify indexes are populated correctly.
			sv := raw.(*structs.SecureVariableEncrypted)
			require.Equal(t, insertIndex, sv.CreateIndex, "incorrect create index", sv.Path)
			require.Equal(t, insertIndex, sv.ModifyIndex, "incorrect modify index", sv.Path)
			// update the mock element so the test element has the correct create/modify
			// indexes and times now that we have validated them
			nv := sv.Copy()
			svm[sv.Path] = &nv
		}
		require.Equal(t, len(svs), count, "incorrect number of secure variables found")
	})
	svs = svm.List()
	t.Log(printSecureVariables(svs))
	t.Run("1a fetch variable", func(t *testing.T) {
		sve, err := testState.GetSecureVariable(ws, svs[0].Namespace, svs[0].Path)
		require.NoError(t, err)
		require.NotNil(t, sve)
	})
	t.Run("2 upsert same", func(t *testing.T) {
		// SubTest Marker: This section attempts to upsert the exact same secure
		// variables without any modification. In this case, the index table
		// should not be updated, indicating no write actually happened due to
		// equality checking.
		reInsertIndex := uint64(30)
		require.NoError(t, testState.UpsertSecureVariables(structs.MsgTypeTestSetup, reInsertIndex, svs))
		reInsertActualIndex, err := testState.Index(TableSecureVariables)
		require.NoError(t, err)
		require.Equal(t, insertIndex, reInsertActualIndex, "index should not have changed")
	})
	t.Run("3 modify one", func(t *testing.T) {
		// SubTest Marker: This section modifies a single one of the previously
		// inserted secure variables and performs an upsert. This ensures the
		// index table is modified correctly and that each secure variable is
		// updated, or not, as expected.
		sv1Update := svs[0].Copy()
		sv1Update.KeyID = "sv1-update"
		svs1Update := []*structs.SecureVariableEncrypted{&sv1Update}

		update1Index := uint64(40)
		require.NoError(t, testState.UpsertSecureVariables(structs.MsgTypeTestSetup, update1Index, svs1Update))

		// Check that the index for the table was modified as expected.
		updateActualIndex, err := testState.Index(TableSecureVariables)
		require.NoError(t, err)
		require.Equal(t, update1Index, updateActualIndex, "index should have changed")

		// Get the secure variables from the table.
		iter, err := testState.SecureVariables(ws)
		require.NoError(t, err)

		// Iterate all the stored registrations and assert they are as expected.
		for raw := iter.Next(); raw != nil; raw = iter.Next() {
			sv := raw.(*structs.SecureVariableEncrypted)
			t.Logf("S " + printSecureVariable(sv))

			var expectedModifyIndex uint64

			switch sv.Path {
			case sv1Update.Path:
				expectedModifyIndex = update1Index
			case svs[1].Path:
				expectedModifyIndex = insertIndex
			default:
				t.Errorf("unknown secure variable found: %s", sv.Path)
				continue
			}
			require.Equal(t, insertIndex, sv.CreateIndex, "incorrect create index", sv.Path)
			require.Equal(t, expectedModifyIndex, sv.ModifyIndex, "incorrect modify index", sv.Path)
			// update the mock element so the test element has the correct create/modify
			// indexes and times now that we have validated them
			nv := sv.Copy()
			svm[sv.Path] = &nv
		}
	})
	svs = svm.List()
	t.Log(printSecureVariables(svs))
	t.Run("4 upsert other", func(t *testing.T) {
		// SubTest Marker: Here we modify the second registration but send an
		// upsert request that includes this and the already modified registration.
		update2Index := uint64(50)
		sv2 := svs[1].Copy()
		sv2.KeyID = "sv2-update"
		sv2.ModifyIndex = update2Index
		svs2Update := []*structs.SecureVariableEncrypted{svs[0], &sv2}
		t.Logf("* " + printSecureVariable(svs[0]))
		t.Logf("* " + printSecureVariable(&sv2))

		require.NoError(t, testState.UpsertSecureVariables(structs.MsgTypeTestSetup, update2Index, svs2Update))

		// Check that the index for the table was modified as expected.
		update2ActualIndex, err := testState.Index(TableSecureVariables)
		require.NoError(t, err)
		require.Equal(t, update2Index, update2ActualIndex, "index should have changed")

		// Get the secure variables registrations from the table.
		iter, err := testState.SecureVariables(ws)
		require.NoError(t, err)

		// Iterate all the stored registrations and assert they are as expected.
		for raw := iter.Next(); raw != nil; raw = iter.Next() {
			sv := raw.(*structs.SecureVariableEncrypted)
			t.Logf("S " + printSecureVariable(sv))

			var (
				expectedModifyIndex uint64
				expectedSV          *structs.SecureVariableEncrypted
			)

			switch sv.Path {
			case sv2.Path:
				expectedModifyIndex = update2Index
				expectedSV = &sv2
			case svs[0].Path:
				expectedModifyIndex = svs[0].ModifyIndex
				expectedSV = svs[0]
			default:
				t.Errorf("unknown secure variable found: %s", sv.Path)
				continue
			}
			require.Equal(t, insertIndex, sv.CreateIndex, "%s: incorrect create index", sv.Path)
			require.Equal(t, expectedModifyIndex, sv.ModifyIndex, "%s: incorrect modify index", sv.Path)

			// update the mock element so the test element has the correct create/modify
			// indexes and times now that we have validated them
			expectedSV.ModifyTime = sv.ModifyTime

			require.True(t, expectedSV.Equals(*sv), "Secure Variables are not equal:\n  expected:%s\n  received:%s\n", printSecureVariable(expectedSV), printSecureVariable(sv))
		}
	})
}

func TestStateStore_DeleteSecureVariable(t *testing.T) {
	ci.Parallel(t)
	testState := testStateStore(t)

	// Generate some test secure variables that we will use and modify throughout.
	svs, _ := mockSecureVariables(2, 2)

	// SubTest Marker: This section attempts to delete a secure variable that
	// does not exist. This is easy to perform here as the state is empty.
	initialIndex := uint64(10)
	err := testState.DeleteSecureVariables(
		structs.MsgTypeTestSetup, initialIndex, svs[0].Namespace, []string{svs[0].Path})
	require.EqualError(t, err, "secure variable not found")

	actualInitialIndex, err := testState.Index(TableSecureVariables)
	require.NoError(t, err)
	require.Equal(t, uint64(0), actualInitialIndex, "index should not have changed")

	// SubTest Marker: This section upserts two secure variables, deletes one,
	// then ensure the remaining is left as expected.
	require.NoError(t, testState.UpsertSecureVariables(structs.MsgTypeTestSetup, initialIndex, svs))

	// Perform the delete.
	delete1Index := uint64(20)
	require.NoError(t, testState.DeleteSecureVariables(
		structs.MsgTypeTestSetup, delete1Index, svs[0].Namespace, []string{svs[0].Path}))

	// Check that the index for the table was modified as expected.
	actualDelete1Index, err := testState.Index(TableSecureVariables)
	require.NoError(t, err)
	require.Equal(t, delete1Index, actualDelete1Index, "index should have changed")

	ws := memdb.NewWatchSet()

	// Get the secure variables from the table.
	iter, err := testState.SecureVariables(ws)
	require.NoError(t, err)

	var delete1Count int

	// Iterate all the stored variables and assert we have the expected
	// number.
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		delete1Count++
	}
	require.Equal(t, 1, delete1Count, "unexpected number of variables in table")

	// SubTest Marker: Delete the remaining variable and ensure all indexes
	// are updated as expected and the table is empty.
	delete2Index := uint64(30)
	require.NoError(t, testState.DeleteSecureVariable(
		delete2Index, svs[1].Namespace, svs[1].Path))

	// Check that the index for the table was modified as expected.
	actualDelete2Index, err := testState.Index(TableSecureVariables)
	require.NoError(t, err)
	require.Equal(t, delete2Index, actualDelete2Index, "index should have changed")

	// Get the secure variables from the table.
	iter, err = testState.SecureVariables(ws)
	require.NoError(t, err)

	var delete2Count int

	// Iterate all the stored registrations and assert we have the expected
	// number.
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		delete2Count++
	}
	require.Equal(t, 0, delete2Count, "unexpected number of variables in table")
}
func TestStateStore_GetSecureVariables(t *testing.T) {
	ci.Parallel(t)
	testState := testStateStore(t)

	// Generate some test secure variables and upsert them.
	svs, _ := mockSecureVariables(2, 2)
	svs[0].Namespace = "~*magical*~"
	initialIndex := uint64(10)
	require.NoError(t, testState.UpsertSecureVariables(structs.MsgTypeTestSetup, initialIndex, svs))

	// Look up secure variables using the namespace of the first mock variable.
	ws := memdb.NewWatchSet()
	iter, err := testState.GetSecureVariablesByNamespace(ws, svs[0].Namespace)
	require.NoError(t, err)

	var count1 int

	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		count1++
		sv := raw.(*structs.SecureVariableEncrypted)
		t.Logf("- sv: n=%q p=%q ci=%v mi=%v ed.ki=%q", sv.Namespace, sv.Path, sv.CreateIndex, sv.ModifyIndex, sv.KeyID)
		require.Equal(t, initialIndex, sv.CreateIndex, "incorrect create index", sv.Path)
		require.Equal(t, initialIndex, sv.ModifyIndex, "incorrect modify index", sv.Path)
		require.Equal(t, svs[0].Namespace, sv.Namespace)
	}
	require.Equal(t, 1, count1)

	// Look up variables using the namespace of the second mock variable.
	iter, err = testState.GetSecureVariablesByNamespace(ws, svs[1].Namespace)
	require.NoError(t, err)

	var count2 int

	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		count2++
		sv := raw.(*structs.SecureVariableEncrypted)
		t.Logf("- sv: n=%q p=%q ci=%v mi=%v ed.ki=%q", sv.Namespace, sv.Path, sv.CreateIndex, sv.ModifyIndex, sv.KeyID)
		require.Equal(t, initialIndex, sv.CreateIndex, "incorrect create index", sv.Path)
		require.Equal(t, initialIndex, sv.ModifyIndex, "incorrect modify index", sv.Path)
		require.Equal(t, svs[1].Namespace, sv.Namespace)
	}
	require.Equal(t, 1, count2)

	// Look up variables using a namespace that shouldn't contain any
	// registrations.
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
	svs, _ := mockSecureVariables(6, 6)
	svs[0].Path = "a/b"
	svs[1].Path = "a/b/c"
	svs[2].Path = "unrelated/b/c"
	svs[3].Namespace = "other"
	svs[3].Path = "a/b/c"
	svs[4].Namespace = "other"
	svs[4].Path = "a/q/z"
	svs[5].Namespace = "other"
	svs[5].Path = "a/z/z"

	initialIndex := uint64(10)
	require.NoError(t, testState.UpsertSecureVariables(structs.MsgTypeTestSetup, initialIndex, svs))

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
					t.Logf("- sv: n=%q p=%q ci=%v mi=%v ed.ki=%q", sv.Namespace, sv.Path, sv.CreateIndex, sv.ModifyIndex, sv.KeyID)
					require.Equal(t, initialIndex, sv.CreateIndex, "incorrect create index", sv.Path)
					require.Equal(t, initialIndex, sv.ModifyIndex, "incorrect modify index", sv.Path)
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
					t.Logf("- sv: n=%q p=%q ci=%v mi=%v ed.ki=%q", sv.Namespace, sv.Path, sv.CreateIndex, sv.ModifyIndex, sv.KeyID)
					require.Equal(t, initialIndex, sv.CreateIndex, "incorrect create index", sv.Path)
					require.Equal(t, initialIndex, sv.ModifyIndex, "incorrect modify index", sv.Path)
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
					t.Logf("- sv: n=%q p=%q ci=%v mi=%v ed.ki=%q", sv.Namespace, sv.Path, sv.CreateIndex, sv.ModifyIndex, sv.KeyID)
					require.Equal(t, initialIndex, sv.CreateIndex, "incorrect create index", sv.Path)
					require.Equal(t, initialIndex, sv.ModifyIndex, "incorrect modify index", sv.Path)
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
	svs, _ := mockSecureVariablesEncrypted(7, 7)
	keyID := uuid.Generate()

	expectedForKey := []string{}
	for i := 0; i < 5; i++ {
		svs[i].KeyID = keyID
		expectedForKey = append(expectedForKey, svs[i].Path)
		sort.Strings(expectedForKey)
	}

	expectedOrphaned := []string{svs[5].Path, svs[6].Path}

	initialIndex := uint64(10)
	require.NoError(t, testState.UpsertSecureVariables(
		structs.MsgTypeTestSetup, initialIndex, svs))

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

// mockSecureVariables returns a random number of secure variables between min
// and max inclusive.
func mockSecureVariables(minU, maxU uint8) (
	[]*structs.SecureVariableEncrypted, secureVariableMocks) {
	// the unsignedness of the args is to prevent goofy parameters, they're
	// easier to work with as ints in this code.
	min := int(minU)
	max := int(maxU)
	vc := min
	// handle cases with irrational arguments. Max < Min = min
	if max > min {
		vc = rand.Intn(max-min) + min
	}
	var svm secureVariableMocks = make(map[string]*structs.SecureVariableEncrypted, vc)
	for i := 0; i < int(vc); i++ {
		nv := mock.SecureVariableEncrypted()
		// There is an extremely rare chance of path collision because the mock
		// secure variables generate their paths randomly. This check will add
		// an extra component on conflict to (ideally) disambiguate them.
		if _, found := svm[nv.Path]; found {
			nv.Path = nv.Path + "/" + fmt.Sprint(time.Now().UnixNano())
		}
		svm[nv.Path] = nv
	}
	return svm.List(), svm
}

type secureVariableMocks map[string]*structs.SecureVariableEncrypted

func (svm secureVariableMocks) List() []*structs.SecureVariableEncrypted {
	out := make([]*structs.SecureVariableEncrypted, len(svm))
	i := 0
	for _, v := range svm {
		out[i] = v
		i++
	}
	// objects will always come out of state store in namespace, path order.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Namespace != out[j].Namespace {
			return out[i].Namespace < out[j].Namespace
		}
		return out[i].Path < out[j].Path
	})
	return out
}

// mockSecureVariables returns a random number of secure variables between min
// and max inclusive.
func mockSecureVariablesEncrypted(minU, maxU uint8) (
	[]*structs.SecureVariableEncrypted, secureVariableMocks) {
	// the unsignedness of the args is to prevent goofy parameters, they're
	// easier to work with as ints in this code.
	min := int(minU)
	max := int(maxU)
	vc := min
	// handle cases with irrational arguments. Max < Min = min
	if max > min {
		vc = rand.Intn(max-min) + min
	}
	var svm secureVariableMocks = make(map[string]*structs.SecureVariableEncrypted, vc)
	for i := 0; i < int(vc); i++ {
		nv := mock.SecureVariableEncrypted()
		// There is an extremely rare chance of path collision because the mock
		// secure variables generate their paths randomly. This check will add
		// an extra component on conflict to (ideally) disambiguate them.
		if _, found := svm[nv.Path]; found {
			nv.Path = nv.Path + "/" + fmt.Sprint(time.Now().UnixNano())
		}
		svm[nv.Path] = nv
	}
	return svm.List(), svm
}

type secureVariableMocksEncrypted map[string]*structs.SecureVariableEncrypted

func (svm secureVariableMocksEncrypted) List() []*structs.SecureVariableEncrypted {
	out := make([]*structs.SecureVariableEncrypted, len(svm))
	i := 0
	for _, v := range svm {
		out[i] = v
		i++
	}
	// objects will always come out of state store in namespace, path order.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Namespace != out[j].Namespace {
			return out[i].Namespace < out[j].Namespace
		}
		return out[i].Path < out[j].Path
	})
	return out
}

func printSecureVariable(tsv *structs.SecureVariableEncrypted) string {
	// b, _ := json.MarshalIndent(tsv, "", "  ")
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
