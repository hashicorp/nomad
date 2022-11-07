package api

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api/internal/testutil"
	"github.com/stretchr/testify/require"
)

func TestVariables_SimpleCRUD(t *testing.T) {
	testutil.Parallel(t)
	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	nsv := c.Variables()
	sv1 := NewVariable("my/first/variable")
	sv1.Namespace = "default"
	sv1.Items["k1"] = "v1"
	sv1.Items["k2"] = "v2"

	sv2 := sv1.Copy()
	sv2.Path = "other/variable/b"
	sv2.Items["k1"] = "otherv1"
	sv2.Items["k2"] = "otherv2"

	t.Run("1 fail create when no items", func(t *testing.T) {

		_, _, err := nsv.Create(&Variable{Path: "bad/var"}, nil)
		require.Error(t, err)
		require.EqualError(t, err, "Unexpected response code: 400 (variable missing required Items object)")
	})

	t.Run("2 create sv1", func(t *testing.T) {

		get, _, err := nsv.Create(sv1, nil)
		require.NoError(t, err)
		require.NotNil(t, get)
		require.NotZero(t, get.CreateIndex)
		require.NotZero(t, get.CreateTime)
		require.NotZero(t, get.ModifyIndex)
		require.NotZero(t, get.ModifyTime)
		require.Equal(t, sv1.Items, get.Items)
		*sv1 = *get
	})

	t.Run("2 create sv2", func(t *testing.T) {

		var err error
		sv2, _, err = nsv.Create(sv2, nil)
		require.NoError(t, err)
	})

	// TODO: Need to prevent no-op modifications from happening server-side
	// t.Run("3 update sv1 no change", func(t *testing.T) {

	// 	get, _, err := nsv.Update(sv1, nil)
	// 	require.NoError(t, err)
	// 	require.NotNil(t, get)
	// 	require.Equal(t, sv1.ModifyIndex, get.ModifyIndex, "ModifyIndex should not change")
	// 	require.Equal(t, sv1.Items, get.Items)
	// 	*sv1 = *get
	// })

	t.Run("4 update sv1", func(t *testing.T) {

		sv1.Items["new-hotness"] = "yeah!"
		get, _, err := nsv.Update(sv1, nil)
		require.NoError(t, err)
		require.NotNil(t, get)
		require.NotEqual(t, sv1.ModifyIndex, get.ModifyIndex, "ModifyIndex should change")
		require.Equal(t, sv1.Items, get.Items)
		*sv1 = *get
	})

	t.Run("5 list vars", func(t *testing.T) {

		l, _, err := nsv.List(nil)
		require.NoError(t, err)
		require.Len(t, l, 2)
		require.ElementsMatch(t, []*VariableMetadata{sv1.Metadata(), sv2.Metadata()}, l)
	})

	t.Run("5a list vars opts", func(t *testing.T) {

		// Since there are two vars in the backend, we should
		// get a NextToken with a page size of 1
		l, qm, err := nsv.List(&QueryOptions{PerPage: 1})
		require.NoError(t, err)
		require.Len(t, l, 1)
		require.Equal(t, sv1.Metadata(), l[0])
		require.NotNil(t, qm.NextToken)
	})

	t.Run("5b prefixlist", func(t *testing.T) {

		l, _, err := nsv.PrefixList("my", nil)
		require.NoError(t, err)
		require.Len(t, l, 1)
		require.Equal(t, sv1.Metadata(), l[0])
	})

	t.Run("6 delete sv1", func(t *testing.T) {

		_, err := nsv.Delete(sv1.Path, nil)
		require.NoError(t, err)
		_, _, err = nsv.Read(sv1.Path, nil)
		require.EqualError(t, err, ErrVariableNotFound)
	})

	t.Run("7 list vars after delete", func(t *testing.T) {

		l, _, err := nsv.List(nil)
		require.NoError(t, err)
		require.NotNil(t, l)
		require.Len(t, l, 1)
	})
}

func TestVariables_CRUDWithCAS(t *testing.T) {
	testutil.Parallel(t)
	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	nsv := c.Variables()
	sv1 := &Variable{
		Path: "cas/variable/a",
		Items: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
	}

	// Create sv1: should pass without issue
	get, _, err := nsv.Create(sv1, nil)
	require.NoError(t, err)
	require.NotNil(t, get)
	require.NotZero(t, get.CreateIndex)
	require.NotZero(t, get.CreateTime)
	require.NotZero(t, get.ModifyIndex)
	require.NotZero(t, get.ModifyTime)
	require.Equal(t, sv1.Items, get.Items)

	// Update sv1 with CAS:

	// - perform out of band upsert
	oobUpdate := sv1.Copy()
	oobUpdate.Items["new-hotness"] = "yeah!"
	nowVal, _, err := nsv.Update(oobUpdate, nil)
	require.NoError(t, err)

	// - try to do an update with sv1's old state; should fail
	_, _, err = nsv.CheckedUpdate(sv1, nil)
	require.Error(t, err)

	// - expect the error to be an ErrCASConflict, so we can cast
	// to it and retrieve the Conflict value
	var conflictErr ErrCASConflict
	require.ErrorAs(t, err, &conflictErr)
	require.Equal(t, nowVal, conflictErr.Conflict)

	// Delete CAS: try to delete sv1 at old ModifyIndex; should
	// return an ErrCASConflict. Check Conflict.
	_, err = nsv.CheckedDelete(sv1.Path, sv1.ModifyIndex, nil)
	require.Error(t, err)
	require.ErrorAs(t, err, &conflictErr)
	require.Equal(t, nowVal, conflictErr.Conflict)

	// Delete CAS: delete at the current index; should succeed.
	_, err = nsv.CheckedDelete(sv1.Path, nowVal.ModifyIndex, nil)
	require.NoError(t, err)

}

func TestVariables_Read(t *testing.T) {
	testutil.Parallel(t)
	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	nsv := c.Variables()
	tID := fmt.Sprint(time.Now().UTC().UnixNano())
	sv1 := Variable{
		Namespace: "default",
		Path:      tID + "/sv1",
		Items: map[string]string{
			"kv1": "val1",
			"kv2": "val2",
		},
	}
	writeTestVariable(t, c, &sv1)

	testCases := []struct {
		name          string
		path          string
		expectedError string
		checkValue    bool
		expectedValue *Variable
	}{
		{
			name:          "not found",
			path:          tID + "/not/found",
			expectedError: ErrVariableNotFound,
			checkValue:    true,
			expectedValue: nil,
		},
		{
			name:          "found",
			path:          sv1.Path,
			checkValue:    true,
			expectedValue: &sv1,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			get, _, err := nsv.Read(tc.path, nil)
			if tc.expectedError != "" {
				require.EqualError(t, err, tc.expectedError)
			} else {
				require.NoError(t, err)
			}
			if tc.checkValue {
				if tc.expectedValue != nil {
					require.NotNil(t, get)
					require.Equal(t, tc.expectedValue, get)
				} else {
					require.Nil(t, get)
				}
			}
		})
	}
}

func writeTestVariable(t *testing.T, c *Client, sv *Variable) {
	_, err := c.write("/v1/var/"+sv.Path, sv, sv, nil)
	require.NoError(t, err, "Error writing test variable")
	require.NoError(t, err, "Error writing test variable")
}

func TestVariable_CreateReturnsContent(t *testing.T) {
	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	nsv := c.Variables()
	sv1 := NewVariable("my/first/variable")
	sv1.Namespace = "default"
	sv1.Items["k1"] = "v1"
	sv1.Items["k2"] = "v2"

	sv1n, _, err := nsv.Create(sv1, nil)
	require.NoError(t, err)
	require.NotNil(t, sv1n)
	require.Equal(t, sv1.Items, sv1n.Items)
}
