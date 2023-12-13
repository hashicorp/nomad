// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/stretchr/testify/require"
)

func TestStructs_VariableDecrypted_Copy(t *testing.T) {
	ci.Parallel(t)
	n := time.Now()
	a := VariableMetadata{
		Namespace:   "a",
		Path:        "a/b/c",
		CreateIndex: 1,
		CreateTime:  n.UnixNano(),
		ModifyIndex: 2,
		ModifyTime:  n.Add(48 * time.Hour).UnixNano(),
	}
	sv := VariableDecrypted{
		VariableMetadata: a,
		Items: VariableItems{
			"foo": "bar",
			"k1":  "v1",
		},
	}
	sv2 := sv.Copy()
	require.True(t, sv.Equal(sv2), "sv and sv2 should be equal")
	sv2.Items["new"] = "new"
	require.False(t, sv.Equal(sv2), "sv and sv2 should not be equal")
}

func TestStructs_VariableDecrypted_Validate(t *testing.T) {
	ci.Parallel(t)

	sv := VariableDecrypted{
		VariableMetadata: VariableMetadata{Namespace: "a"},
		Items:            VariableItems{"foo": "bar"},
	}

	testCases := []struct {
		path string
		ok   bool
	}{
		{path: ""},
		{path: "nomad"},
		{path: "nomad/other"},
		{path: "a/b/c", ok: true},
		{path: "nomad/jobs", ok: true},
		{path: "nomadjobs", ok: true},
		{path: "nomad/jobs/whatever", ok: true},
		{path: "example/_-~/whatever", ok: true},
		{path: "example/@whatever"},
		{path: "example/what.ever"},
		{path: "nomad/job-templates"},
		{path: "nomad/job-templates/whatever", ok: true},
	}
	for _, tc := range testCases {
		tc := tc
		sv.Path = tc.path
		err := sv.Validate()
		if tc.ok {
			require.NoError(t, err, "should not get error for: %s", tc.path)
		} else {
			require.Error(t, err, "should get error for: %s", tc.path)
		}
	}
}
