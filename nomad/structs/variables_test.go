package structs

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/stretchr/testify/require"
)

func TestStructs_SecureVariableDecrypted_Copy(t *testing.T) {
	ci.Parallel(t)
	n := time.Now()
	a := SecureVariableMetadata{
		Namespace:   "a",
		Path:        "a/b/c",
		CreateIndex: 1,
		CreateTime:  n.UnixNano(),
		ModifyIndex: 2,
		ModifyTime:  n.Add(48 * time.Hour).UnixNano(),
	}
	sv := SecureVariableDecrypted{
		SecureVariableMetadata: a,
		Items: SecureVariableItems{
			"foo": "bar",
			"k1":  "v1",
		},
	}
	sv2 := sv.Copy()
	require.True(t, sv.Equals(sv2), "sv and sv2 should be equal")
	sv2.Items["new"] = "new"
	require.False(t, sv.Equals(sv2), "sv and sv2 should not be equal")
}

func TestStructs_SecureVariableDecrypted_Validate(t *testing.T) {
	ci.Parallel(t)

	sv := SecureVariableDecrypted{
		SecureVariableMetadata: SecureVariableMetadata{Namespace: "a"},
		Items:                  SecureVariableItems{"foo": "bar"},
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
