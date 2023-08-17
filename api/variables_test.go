// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api/internal/testutil"
	"github.com/shoenig/test/must"
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
		must.ErrorContains(t, err, "Unexpected response code: 400 (variable missing required Items object)")
	})

	t.Run("2 create sv1", func(t *testing.T) {
		get, _, err := nsv.Create(sv1, nil)
		must.NoError(t, err)
		must.NotNil(t, get)
		must.Positive(t, get.CreateIndex)
		must.Positive(t, get.CreateTime)
		must.Positive(t, get.ModifyIndex)
		must.Positive(t, get.ModifyTime)
		must.Eq(t, sv1.Items, get.Items)
		*sv1 = *get
	})

	t.Run("2 create sv2", func(t *testing.T) {

		var err error
		sv2, _, err = nsv.Create(sv2, nil)
		must.NoError(t, err)
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
		must.NoError(t, err)
		must.NotNil(t, get)
		must.NotEq(t, sv1.ModifyIndex, get.ModifyIndex)
		must.Eq(t, sv1.Items, get.Items)
		*sv1 = *get
	})

	t.Run("5 list vars", func(t *testing.T) {
		l, _, err := nsv.List(nil)
		must.NoError(t, err)
		must.Len(t, 2, l)
		must.Eq(t, []*VariableMetadata{sv1.Metadata(), sv2.Metadata()}, l)
	})

	t.Run("5a list vars opts", func(t *testing.T) {
		// Since there are two vars in the backend, we should
		// get a NextToken with a page size of 1
		l, qm, err := nsv.List(&QueryOptions{PerPage: 1})
		must.NoError(t, err)
		must.Len(t, 1, l)
		must.Eq(t, sv1.Metadata(), l[0])
		must.NotNil(t, qm.NextToken)
	})

	t.Run("5b prefixlist", func(t *testing.T) {
		l, _, err := nsv.PrefixList("my", nil)
		must.NoError(t, err)
		must.Len(t, 1, l)
		must.Eq(t, sv1.Metadata(), l[0])
	})

	t.Run("6 delete sv1", func(t *testing.T) {
		_, err := nsv.Delete(sv1.Path, nil)
		must.NoError(t, err)
		_, _, err = nsv.Read(sv1.Path, nil)
		must.ErrorIs(t, err, ErrVariablePathNotFound)
	})

	t.Run("7 list vars after delete", func(t *testing.T) {
		l, _, err := nsv.List(nil)
		must.NoError(t, err)
		must.NotNil(t, l)
		must.Len(t, 1, l)
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
	must.NoError(t, err)
	must.NotNil(t, get)
	must.Positive(t, get.CreateIndex)
	must.Positive(t, get.CreateTime)
	must.Positive(t, get.ModifyIndex)
	must.Positive(t, get.ModifyTime)
	must.Eq(t, sv1.Items, get.Items)

	// Update sv1 with CAS:

	// - perform out of band upsert
	oobUpdate := sv1.Copy()
	oobUpdate.Items["new-hotness"] = "yeah!"
	nowVal, _, err := nsv.Update(oobUpdate, nil)
	must.NoError(t, err)

	// - try to do an update with sv1's old state; should fail
	_, _, err = nsv.CheckedUpdate(sv1, nil)
	must.Error(t, err)

	// - expect the error to be an ErrCASConflict, so we can cast
	// to it and retrieve the Conflict value
	var conflictErr ErrCASConflict
	must.True(t, errors.As(err, &conflictErr))
	must.Eq(t, nowVal, conflictErr.Conflict)

	// Delete CAS: try to delete sv1 at old ModifyIndex; should
	// return an ErrCASConflict. Check Conflict.
	_, err = nsv.CheckedDelete(sv1.Path, sv1.ModifyIndex, nil)
	must.True(t, errors.As(err, &conflictErr))
	must.Eq(t, nowVal, conflictErr.Conflict)

	// Delete CAS: delete at the current index; should succeed.
	_, err = nsv.CheckedDelete(sv1.Path, nowVal.ModifyIndex, nil)
	must.NoError(t, err)
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
		expectedError error
		checkValue    bool
		expectedValue *Variable
	}{
		{
			name:          "not found",
			path:          tID + "/not/found",
			expectedError: ErrVariablePathNotFound,
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
			if tc.expectedError != nil {
				must.ErrorIs(t, err, tc.expectedError)
			} else {
				must.NoError(t, err)
			}
			if tc.checkValue {
				if tc.expectedValue != nil {
					must.NotNil(t, get)
					must.Eq(t, tc.expectedValue, get)
				} else {
					must.Nil(t, get)
				}
			}
		})
	}
}

func TestVariables_GetVariableItems(t *testing.T) {
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
		expectedError error
		checkValue    bool
		expectedValue VariableItems
	}{
		{
			name:          "not found",
			path:          tID + "/not/found",
			expectedError: ErrVariablePathNotFound,
			checkValue:    true,
			expectedValue: nil,
		},
		{
			name:          "found",
			path:          sv1.Path,
			checkValue:    true,
			expectedValue: sv1.Items,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			get, _, err := nsv.GetVariableItems(tc.path, nil)
			if tc.expectedError != nil {
				must.ErrorIs(t, err, tc.expectedError)
			} else {
				must.NoError(t, err)
			}
			if tc.checkValue {
				if tc.expectedValue != nil {
					must.NotNil(t, get)
					must.Eq(t, tc.expectedValue, get)
				} else {
					must.Nil(t, get)
				}
			}
		})
	}
}

func writeTestVariable(t *testing.T, c *Client, sv *Variable) {
	_, err := c.put("/v1/var/"+sv.Path, sv, sv, nil)
	must.NoError(t, err, must.Sprint("failed writing test variable"))
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
	must.NoError(t, err)
	must.NotNil(t, sv1n)
	must.Eq(t, sv1.Items, sv1n.Items)
}

func TestVariables_LockRenewRelease(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	nsv := c.Variables()
	sv1 := NewVariable("my/first/variable")
	sv1.Namespace = "default"
	sv1.Items["k1"] = "v1"
	sv1.Items["k2"] = "v2"

	t.Run("2 create sv1", func(t *testing.T) {
		get, _, err := nsv.Create(sv1, nil)
		must.NoError(t, err)
		must.NotNil(t, get)
		must.Positive(t, get.CreateIndex)
		must.Positive(t, get.CreateTime)
		must.Positive(t, get.ModifyIndex)
		must.Positive(t, get.ModifyTime)
		must.Eq(t, sv1.Items, get.Items)
		*sv1 = *get
	})

	t.Run("3 acquire lock on sv1", func(t *testing.T) {
		get, _, err := nsv.AcquireLock(sv1, nil)
		must.NoError(t, err)
		must.NotNil(t, get)
		must.NotEq(t, sv1.ModifyIndex, get.ModifyIndex)
		must.Eq(t, sv1.Items, get.Items)
		must.NotNil(t, get.Lock)

		*sv1 = *get
	})

	t.Run("4 renew lock on sv1", func(t *testing.T) {
		get, _, err := nsv.RenewLock(sv1, nil)
		must.NoError(t, err)
		must.NotNil(t, get)
		must.Eq(t, sv1.ModifyIndex, get.ModifyIndex)
		must.NotNil(t, get.Lock)
		must.Eq(t, sv1.Lock.ID, get.Lock.ID)
	})

	t.Run("5 list vars", func(t *testing.T) {
		l, _, err := nsv.List(nil)
		must.NoError(t, err)
		must.Len(t, 1, l)
		must.NotNil(t, l[0].Lock)
		must.Eq(t, sv1.Lock.ID, l[0].Lock.ID)
	})

	t.Run("6 release lock on sv1", func(t *testing.T) {
		get, _, err := nsv.ReleaseLock(sv1, nil)
		must.NoError(t, err)
		must.NotNil(t, get)
		must.NotEq(t, sv1.ModifyIndex, get.ModifyIndex)
		must.Eq(t, sv1.Items, get.Items)
		must.Nil(t, get.Lock)

		*sv1 = *get
	})
}
