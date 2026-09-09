// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MPL-2.0

package jobspec2

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/shoenig/test/must"
)

func TestParse_GroupSelection(t *testing.T) {
	t.Parallel()
	source, err := os.ReadFile("test-fixtures/group-selection.hcl")
	must.NoError(t, err)
	job, err := ParseWithConfig(&ParseConfig{Path: "test-fixtures/group-selection.hcl", Body: source, Strict: true})
	must.NoError(t, err)
	job.Canonicalize()
	must.Len(t, 1, job.GroupSelections)
	selection := job.GroupSelections[0]
	must.Eq(t, "runtime", selection.Name)
	must.Eq(t, 2, *selection.Count)
	must.Eq(t, []string{"encoder", "orin", "thor"}, selection.Groups)
	for i, want := range []int{2, 3, 5} {
		must.Eq(t, want, *job.TaskGroups[i].Count)
	}
	for i, want := range []int{1000, 800, 500} {
		must.Eq(t, want, *job.TaskGroups[i].Tasks[0].Resources.CPU)
		must.Eq(t, want, *job.TaskGroups[i].Tasks[0].Resources.MemoryMB)
	}
	// The normal HCL-to-JSON submission path must preserve group selection and
	// leave each group's resource and replica requirements untouched.
	encoded, err := json.Marshal(job)
	must.NoError(t, err)
	var decoded api.Job
	must.NoError(t, json.Unmarshal(encoded, &decoded))
	must.Eq(t, job.GroupSelections, decoded.GroupSelections)
	must.Eq(t, job.TaskGroups, decoded.TaskGroups)
}

func TestParse_GroupSelectionDefaults(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		count      int
	}{
		{"default", `job "example" {
 group_selection "runtime" { groups = ["a", "b"] }
}`, 1},
		{"zero", `job "example" {
 group_selection "runtime" {
   count = 0
   groups = ["a", "b"]
  }
 }`, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			job, err := ParseWithConfig(&ParseConfig{Path: "input.hcl", Body: []byte(tc.body), Strict: true})
			must.NoError(t, err)
			job.Canonicalize()
			must.Eq(t, tc.count, *job.GroupSelections[0].Count)
		})
	}
	_, err := ParseWithConfig(&ParseConfig{Path: "input.hcl", Body: []byte(`job "example" {
 group_selection "runtime" { count = 1 }
}`), Strict: true})
	must.Error(t, err)
}
