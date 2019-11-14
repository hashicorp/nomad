package state

import (
	"testing"

	memdb "github.com/hashicorp/go-memdb"
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
