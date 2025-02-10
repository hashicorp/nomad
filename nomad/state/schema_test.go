// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"errors"
	"testing"

	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/require"
)

func TestStateStoreSchema(t *testing.T) {
	ci.Parallel(t)

	schema := stateStoreSchema()
	_, err := memdb.NewMemDB(schema)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestState_singleRecord(t *testing.T) {
	ci.Parallel(t)

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

func Test_jobIsGCable(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name                string
		inputObj            interface{}
		expectedOutput      bool
		expectedOutputError error
	}{
		{
			name:                "not a job object",
			inputObj:            &structs.Node{},
			expectedOutput:      false,
			expectedOutputError: errors.New("Unexpected type:"),
		},
		{
			name: "stopped periodic",
			inputObj: &structs.Job{
				Stop:     true,
				Periodic: &structs.PeriodicConfig{Enabled: true},
			},
			expectedOutput:      true,
			expectedOutputError: nil,
		},
		{
			name: "stopped parameterized",
			inputObj: &structs.Job{
				Stop:             true,
				ParameterizedJob: &structs.ParameterizedJobConfig{},
			},
			expectedOutput:      true,
			expectedOutputError: nil,
		},
		{
			name: "running periodic",
			inputObj: &structs.Job{
				Stop:     false,
				Periodic: &structs.PeriodicConfig{Enabled: true},
			},
			expectedOutput:      false,
			expectedOutputError: nil,
		},
		{
			name: "running parameterized",
			inputObj: &structs.Job{
				Stop:             false,
				ParameterizedJob: &structs.ParameterizedJobConfig{},
			},
			expectedOutput:      false,
			expectedOutputError: nil,
		},
		{
			name: "running service",
			inputObj: &structs.Job{
				Type:   structs.JobTypeService,
				Status: structs.JobStatusRunning,
			},
			expectedOutput:      false,
			expectedOutputError: nil,
		},
		{
			name: "running batch",
			inputObj: &structs.Job{
				Type:   structs.JobTypeBatch,
				Status: structs.JobStatusRunning,
			},
			expectedOutput:      false,
			expectedOutputError: nil,
		},
		{
			name: "running system",
			inputObj: &structs.Job{
				Type:   structs.JobTypeSystem,
				Status: structs.JobStatusRunning,
			},
			expectedOutput:      false,
			expectedOutputError: nil,
		},
		{
			name: "running sysbatch",
			inputObj: &structs.Job{
				Type:   structs.JobTypeSysBatch,
				Status: structs.JobStatusRunning,
			},
			expectedOutput:      false,
			expectedOutputError: nil,
		},
		{
			name: "user stopped service",
			inputObj: &structs.Job{
				Type:   structs.JobTypeService,
				Status: structs.JobStatusDead,
				Stop:   true,
			},
			expectedOutput:      true,
			expectedOutputError: nil,
		},
		{
			name: "user stopped batch",
			inputObj: &structs.Job{
				Type:   structs.JobTypeBatch,
				Status: structs.JobStatusDead,
				Stop:   true,
			},
			expectedOutput:      true,
			expectedOutputError: nil,
		},
		{
			name: "user stopped system",
			inputObj: &structs.Job{
				Type:   structs.JobTypeSystem,
				Status: structs.JobStatusDead,
				Stop:   true,
			},
			expectedOutput:      true,
			expectedOutputError: nil,
		},
		{
			name: "user stopped sysbatch",
			inputObj: &structs.Job{
				Type:   structs.JobTypeSysBatch,
				Status: structs.JobStatusDead,
				Stop:   true,
			},
			expectedOutput:      true,
			expectedOutputError: nil,
		},
		{
			name: "non-user stopped batch",
			inputObj: &structs.Job{
				Type:   structs.JobTypeBatch,
				Status: structs.JobStatusDead,
			},
			expectedOutput:      true,
			expectedOutputError: nil,
		},
		{
			name: "non-user stopped sysbatch",
			inputObj: &structs.Job{
				Type:   structs.JobTypeSysBatch,
				Status: structs.JobStatusDead,
			},
			expectedOutput:      true,
			expectedOutputError: nil,
		},
		{
			name: "tagged",
			inputObj: &structs.Job{
				VersionTag: &structs.JobVersionTag{Name: "any"},
			},
			expectedOutput:      false,
			expectedOutputError: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			actualOutput, actualError := jobIsGCable(tc.inputObj)
			must.Eq(t, tc.expectedOutput, actualOutput)

			if tc.expectedOutputError != nil {
				must.ErrorContains(t, actualError, tc.expectedOutputError.Error())
			} else {
				must.NoError(t, actualError)
			}
		})
	}
}

func TestState_ScalingPolicyTargetFieldIndex_FromObject(t *testing.T) {
	ci.Parallel(t)

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
