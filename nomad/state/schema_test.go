package state

import (
	"github.com/hashicorp/nomad/nomad/structs"
	"testing"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/stretchr/testify/require"
)

func TestStateStoreSchema(t *testing.T) {
	schema := stateStoreSchema()
	_, err := memdb.NewMemDB(schema)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestState_singleRecord(t *testing.T) {
	require := require.New(t)

	const (
		singletonTable = "cluster_meta"
		singletonIDIdx = "id"
	)

	db, err := memdb.NewMemDB(&memdb.DBSchema{
		Tables: map[string]*memdb.TableSchema{
			singletonTable: clusterMetaTableSchema(),
		},
	})
	require.NoError(err)

	// numRecords in table counts all the items in the table, which is expected
	// to always be 1 since that's the point of the singletonRecord Indexer.
	numRecordsInTable := func() int {
		txn := db.Txn(false)
		defer txn.Abort()

		iter, err := txn.Get(singletonTable, singletonIDIdx)
		require.NoError(err)

		num := 0
		for item := iter.Next(); item != nil; item = iter.Next() {
			num++
		}
		return num
	}

	// setSingleton "updates" the singleton record in the singletonTable,
	// which requires that the singletonRecord Indexer is working as
	// expected.
	setSingleton := func(s string) {
		txn := db.Txn(true)
		err := txn.Insert(singletonTable, s)
		require.NoError(err)
		txn.Commit()
	}

	// first retrieves the one expected entry in the singletonTable - use the
	// numRecordsInTable helper function to make the cardinality assertion,
	// this is just for fetching the value.
	first := func() string {
		txn := db.Txn(false)
		defer txn.Abort()
		record, err := txn.First(singletonTable, singletonIDIdx)
		require.NoError(err)
		s, ok := record.(string)
		require.True(ok)
		return s
	}

	// Ensure that multiple Insert & Commit calls result in only
	// a single "singleton" record existing in the table.

	setSingleton("one")
	require.Equal(1, numRecordsInTable())
	require.Equal("one", first())

	setSingleton("two")
	require.Equal(1, numRecordsInTable())
	require.Equal("two", first())

	setSingleton("three")
	require.Equal(1, numRecordsInTable())
	require.Equal("three", first())
}

func TestState_ScalingPolicyTargetFieldIndex_FromObject(t *testing.T) {
	require := require.New(t)

	policy := mock.ScalingPolicy()
	policy.Target["TestField"] = "test"

	// Create test indexers
	indexersAllowMissingTrue := &ScalingPolicyTargetFieldIndex{Field: "TestField", AllowMissing: true}
	indexersAllowMissingFalse := &ScalingPolicyTargetFieldIndex{Field: "TestField", AllowMissing: false}

	// Check if box indexers can find the test field
	ok, val, err := indexersAllowMissingTrue.FromObject(policy)
	require.True(ok)
	require.NoError(err)
	require.Equal("test\x00", string(val))

	ok, val, err = indexersAllowMissingFalse.FromObject(policy)
	require.True(ok)
	require.NoError(err)
	require.Equal("test\x00", string(val))

	// Check for empty field
	policy.Target["TestField"] = ""

	ok, val, err = indexersAllowMissingTrue.FromObject(policy)
	require.True(ok)
	require.NoError(err)
	require.Equal("\x00", string(val))

	ok, val, err = indexersAllowMissingFalse.FromObject(policy)
	require.True(ok)
	require.NoError(err)
	require.Equal("\x00", string(val))

	// Check for missing field
	delete(policy.Target, "TestField")

	ok, val, err = indexersAllowMissingTrue.FromObject(policy)
	require.True(ok)
	require.NoError(err)
	require.Equal("\x00", string(val))

	ok, val, err = indexersAllowMissingFalse.FromObject(policy)
	require.False(ok)
	require.NoError(err)
	require.Equal("", string(val))

	// Check for invalid input
	ok, val, err = indexersAllowMissingTrue.FromObject("not-a-scaling-policy")
	require.False(ok)
	require.Error(err)
	require.Equal("", string(val))

	ok, val, err = indexersAllowMissingFalse.FromObject("not-a-scaling-policy")
	require.False(ok)
	require.Error(err)
	require.Equal("", string(val))
}

func TestEventTableUintIndex(t *testing.T) {

	require := require.New(t)

	const (
		eventsTable = "events"
		uintIDIdx   = "id"
	)

	db, err := memdb.NewMemDB(&memdb.DBSchema{
		Tables: map[string]*memdb.TableSchema{
			eventsTable: eventTableSchema(),
		},
	})
	require.NoError(err)

	// numRecords in table counts all the items in the table, which is expected
	// to always be 1 since that's the point of the singletonRecord Indexer.
	numRecordsInTable := func() int {
		txn := db.Txn(false)
		defer txn.Abort()

		iter, err := txn.Get(eventsTable, uintIDIdx)
		require.NoError(err)

		num := 0
		for item := iter.Next(); item != nil; item = iter.Next() {
			num++
		}
		return num
	}

	insertEvents := func(e *structs.Events) {
		txn := db.Txn(true)
		err := txn.Insert(eventsTable, e)
		require.NoError(err)
		txn.Commit()
	}

	get := func(idx uint64) *structs.Events {
		txn := db.Txn(false)
		defer txn.Abort()
		record, err := txn.First("events", "id", idx)
		require.NoError(err)
		s, ok := record.(*structs.Events)
		require.True(ok)
		return s
	}

	firstEvent := &structs.Events{Index: 10, Events: []structs.Event{{Index: 10}, {Index: 10}}}
	secondEvent := &structs.Events{Index: 11, Events: []structs.Event{{Index: 11}, {Index: 11}}}
	thirdEvent := &structs.Events{Index: 202, Events: []structs.Event{{Index: 202}, {Index: 202}}}
	insertEvents(firstEvent)
	insertEvents(secondEvent)
	insertEvents(thirdEvent)
	require.Equal(3, numRecordsInTable())

	gotFirst := get(10)
	require.Equal(firstEvent, gotFirst)

	gotSecond := get(11)
	require.Equal(secondEvent, gotSecond)

	gotThird := get(202)
	require.Equal(thirdEvent, gotThird)
}
