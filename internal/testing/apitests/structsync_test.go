// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package apitests

import (
	"encoding/json"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

// Tests that api and struct values are equivalent
//
// Given that vendoring libraries prune tests by default, test dependencies
// aren't leaked to clients of the package - so it should be safe to add
// such dependency without affecting api clients.

func TestDefaultResourcesAreInSync(t *testing.T) {
	ci.Parallel(t)

	apiR := api.DefaultResources()
	structsR := structs.DefaultResources()

	require.EqualValues(t, *structsR, toStructsResource(t, apiR))

	// match after canonicalization
	apiR.Canonicalize()
	structsR.Canonicalize()
	require.EqualValues(t, *structsR, toStructsResource(t, apiR))
}

func TestMinResourcesAreInSync(t *testing.T) {
	ci.Parallel(t)

	apiR := api.MinResources()
	structsR := structs.MinResources()

	require.EqualValues(t, *structsR, toStructsResource(t, apiR))

	// match after canonicalization
	apiR.Canonicalize()
	structsR.Canonicalize()
	require.EqualValues(t, *structsR, toStructsResource(t, apiR))
}

func TestNewDefaultRescheulePolicyInSync(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		typ      string
		expected structs.ReschedulePolicy
	}{
		{"service", structs.DefaultServiceJobReschedulePolicy},
		{"batch", structs.DefaultBatchJobReschedulePolicy},
		{"system", structs.ReschedulePolicy{}},
	}

	for _, c := range cases {
		t.Run(c.typ, func(t *testing.T) {
			apiP := api.NewDefaultReschedulePolicy(c.typ)

			var found structs.ReschedulePolicy
			toStructs(t, &found, apiP)

			require.EqualValues(t, c.expected, found)
		})
	}
}

func TestNewDefaultRestartPolicyInSync(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		typ      string
		expected structs.RestartPolicy
	}{
		{"service", structs.DefaultServiceJobRestartPolicy},
		{"batch", structs.DefaultBatchJobRestartPolicy},
		{"system", structs.DefaultServiceJobRestartPolicy},
	}

	for _, c := range cases {
		t.Run(c.typ, func(t *testing.T) {
			job := api.Job{Type: &c.typ}
			var tg api.TaskGroup
			tg.Canonicalize(&job)

			apiP := tg.RestartPolicy

			var found structs.RestartPolicy
			toStructs(t, &found, apiP)

			require.EqualValues(t, c.expected, found)
		})
	}
}

func toStructsResource(t *testing.T, in *api.Resources) structs.Resources {
	var out structs.Resources
	toStructs(t, &out, in)
	return out
}

func toStructs(t *testing.T, out, in interface{}) {
	bytes, err := json.Marshal(in)
	require.NoError(t, err)

	err = json.Unmarshal(bytes, &out)
	require.NoError(t, err)
}
