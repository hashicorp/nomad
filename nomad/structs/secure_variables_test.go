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
		CreateTime:  n,
		ModifyIndex: 2,
		ModifyTime:  n.Add(48 * time.Hour),
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
