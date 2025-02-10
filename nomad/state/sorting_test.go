// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func TestGetSorted(t *testing.T) {
	store, err := NewStateStore(&StateStoreConfig{
		JobTrackedVersions: 1,

		Logger: hclog.L().Named("TestGetSorted"),
	})
	must.NoError(t, err)

	jobs := make([]*structs.Job, 3)
	jobs[0] = mock.Job()
	jobs[0].ID = "ayyy"
	jobs[1] = mock.Job()
	jobs[1].ID = "beee"
	jobs[2] = mock.Job()
	jobs[2].ID = "ceee"

	txn := store.db.WriteTxn(100)
	for _, j := range jobs {
		must.NoError(t, txn.Insert("jobs", j))
	}
	must.NoError(t, txn.Commit())

	for _, tc := range []struct {
		name    string
		reverse bool
		expect  []string
	}{
		// with jobs "id" index, they should be in lexicographical order by ID
		{"default", false, []string{"ayyy", "beee", "ceee"}},
		{"reverse", true, []string{"ceee", "beee", "ayyy"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			txn = store.db.ReadTxn()

			// also tangentially test QueryOptionSort
			sort := QueryOptionSort(structs.QueryOptions{
				Reverse: tc.reverse,
			})

			// method under test
			iter, err := getSorted(txn, sort, "jobs", "id")
			must.NoError(t, err)

			got := make([]string, len(jobs))
			for x, _ := range jobs {
				raw := iter.Next()
				job := raw.(*structs.Job)
				got[x] = job.ID
			}

			must.Eq(t, tc.expect, got)
		})
	}
}
