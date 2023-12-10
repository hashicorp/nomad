// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
)

func TestVariableMetadata_Equal(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name            string
		inputMetadata   VariableMetadata
		inputMetadataFn VariableMetadata
		expectedOutput  bool
	}{
		{
			name: "no lock equal",
			inputMetadata: VariableMetadata{
				Namespace:   "default",
				Path:        "custom/test/path",
				Lock:        nil,
				CreateIndex: 10,
				CreateTime:  1687251815,
				ModifyIndex: 100,
				ModifyTime:  1687251868,
			},
			inputMetadataFn: VariableMetadata{
				Namespace:   "default",
				Path:        "custom/test/path",
				Lock:        nil,
				CreateIndex: 10,
				CreateTime:  1687251815,
				ModifyIndex: 100,
				ModifyTime:  1687251868,
			},
			expectedOutput: true,
		},
		{
			name: "no lock unequal",
			inputMetadata: VariableMetadata{
				Namespace:   "default",
				Path:        "custom/test/path",
				Lock:        nil,
				CreateIndex: 10,
				CreateTime:  1687251815,
				ModifyIndex: 100,
				ModifyTime:  1687251876,
			},
			inputMetadataFn: VariableMetadata{
				Namespace:   "default",
				Path:        "custom/test/path",
				Lock:        nil,
				CreateIndex: 10,
				CreateTime:  1687251815,
				ModifyIndex: 200,
				ModifyTime:  1687251885,
			},
			expectedOutput: false,
		},

		{
			name: "lock equal",
			inputMetadata: VariableMetadata{
				Namespace: "default",
				Path:      "custom/test/path",
				Lock: &VariableLock{
					ID:        "896bdbef-8ce7-4b1d-9b4c-4e6c5639196d",
					TTL:       20 * time.Second,
					LockDelay: 5 * time.Second,
				},
				CreateIndex: 10,
				CreateTime:  1687251815,
				ModifyIndex: 100,
				ModifyTime:  1687251868,
			},
			inputMetadataFn: VariableMetadata{
				Namespace: "default",
				Path:      "custom/test/path",
				Lock: &VariableLock{
					ID:        "896bdbef-8ce7-4b1d-9b4c-4e6c5639196d",
					TTL:       20 * time.Second,
					LockDelay: 5 * time.Second,
				},
				CreateIndex: 10,
				CreateTime:  1687251815,
				ModifyIndex: 100,
				ModifyTime:  1687251868,
			},
			expectedOutput: true,
		},
		{
			name: "lock unequal",
			inputMetadata: VariableMetadata{
				Namespace: "default",
				Path:      "custom/test/path",
				Lock: &VariableLock{
					ID:        "896bdbef-8ce7-4b1d-9b4c-4e6c5639196d",
					TTL:       20 * time.Second,
					LockDelay: 5 * time.Second,
				},
				CreateIndex: 10,
				CreateTime:  1687251815,
				ModifyIndex: 100,
				ModifyTime:  1687251876,
			},
			inputMetadataFn: VariableMetadata{
				Namespace: "default",
				Path:      "custom/test/path",
				Lock: &VariableLock{
					ID:        "896bdbef-8ce7-4b1d-9b4c-4e6c5639196d",
					TTL:       20 * time.Second,
					LockDelay: 15 * time.Second,
				},
				CreateIndex: 10,
				CreateTime:  1687251815,
				ModifyIndex: 100,
				ModifyTime:  1687251876,
			},
			expectedOutput: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			must.Eq(t, tc.expectedOutput, tc.inputMetadata.Equal(tc.inputMetadataFn))
		})
	}
}

func TestVariableMetadata_Copy(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name                  string
		inputVariableMetadata *VariableMetadata
	}{
		{
			name: "no lock",
			inputVariableMetadata: &VariableMetadata{
				Namespace:   "default",
				Path:        "custom/test/path",
				Lock:        nil,
				CreateIndex: 10,
				CreateTime:  1687251815,
				ModifyIndex: 100,
				ModifyTime:  1687251876,
			},
		},
		{
			name: "lock",
			inputVariableMetadata: &VariableMetadata{
				Namespace: "default",
				Path:      "custom/test/path",
				Lock: &VariableLock{
					ID:        "896bdbef-8ce7-4b1d-9b4c-4e6c5639196d",
					TTL:       20 * time.Second,
					LockDelay: 15 * time.Second,
				},
				CreateIndex: 10,
				CreateTime:  1687251815,
				ModifyIndex: 100,
				ModifyTime:  1687251876,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := tc.inputVariableMetadata.Copy()
			must.Eq(t, tc.inputVariableMetadata, actualOutput)
			must.NotEqOp(t,
				fmt.Sprintf("%p", tc.inputVariableMetadata),
				fmt.Sprintf("%p", actualOutput))

			if tc.inputVariableMetadata.Lock != nil {
				must.NotEqOp(t,
					fmt.Sprintf("%p", tc.inputVariableMetadata.Lock),
					fmt.Sprintf("%p", actualOutput.Lock))
			}
		})
	}
}

func TestVariableMetadata_LockID(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name                  string
		inputVariableMetadata *VariableMetadata
		expectedOutput        string
	}{
		{
			name: "nil lock",
			inputVariableMetadata: &VariableMetadata{
				Lock: nil,
			},
			expectedOutput: "",
		},
		{
			name: "empty ID",
			inputVariableMetadata: &VariableMetadata{
				Lock: &VariableLock{ID: ""},
			},
			expectedOutput: "",
		},
		{
			name: "populated ID",
			inputVariableMetadata: &VariableMetadata{
				Lock: &VariableLock{ID: "mylovelylovelyid"},
			},
			expectedOutput: "mylovelylovelyid",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			must.Eq(t, tc.expectedOutput, tc.inputVariableMetadata.LockID())
		})
	}
}

func TestVariableMetadata_IsLock(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name                  string
		inputVariableMetadata *VariableMetadata
		expectedOutput        bool
	}{
		{
			name: "nil",
			inputVariableMetadata: &VariableMetadata{
				Lock: nil,
			},
			expectedOutput: false,
		},
		{
			name: "not nil",
			inputVariableMetadata: &VariableMetadata{
				Lock: &VariableLock{},
			},
			expectedOutput: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			must.Eq(t, tc.expectedOutput, tc.inputVariableMetadata.IsLock())
		})
	}
}

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
	must.True(t, sv.Equal(sv2), must.Sprintf("sv and sv2 should be equal"))
	sv2.Items["new"] = "new"
	must.False(t, sv.Equal(sv2), must.Sprintf("sv and sv2 should not be equal"))
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
			must.NoError(t, err, must.Sprintf("should not get error for: %s", tc.path))
		} else {
			must.Error(t, err, must.Sprintf("should get error for: %s", tc.path))
		}
	}
}

func TestStructs_VariablesRenewLockRequest_Validate(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name    string
		request *VariablesRenewLockRequest
		expErr  error
	}{
		{
			name: "missing_lockID",
			request: &VariablesRenewLockRequest{
				Path: "path",
			},
			expErr: errNoLock,
		},
		{
			name: "missing_path",
			request: &VariablesRenewLockRequest{
				LockID: "lockID",
			},
			expErr: errNoPath,
		},
		{
			name: "valid_request",
			request: &VariablesRenewLockRequest{
				Path:   "path",
				LockID: "lockID",
			},
			expErr: nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.request.Validate()
			if !errors.Is(err, tc.expErr) {
				t.Errorf("Expected error %v, but got error %v", tc.expErr, err)
			}
		})
	}
}

func TestStructs_Lock_Validate(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name   string
		lock   *VariableLock
		expErr error
	}{
		{
			name: "lock_delay_is_negative",
			lock: &VariableLock{
				TTL:       5 * time.Second,
				LockDelay: -5 * time.Second,
			},
			expErr: errNegativeDelayOrTTL,
		},
		{
			name: "lock_ttl_is_negative",
			lock: &VariableLock{
				TTL:       -5 * time.Second,
				LockDelay: 5 * time.Second,
			},
			expErr: errNegativeDelayOrTTL,
		},
		{
			name: "lock_ttl_is_bigger_than_max",
			lock: &VariableLock{
				TTL:       maxVariableLockTTL + 5*time.Second,
				LockDelay: 5 * time.Second,
			},
			expErr: errInvalidTTL,
		},
		{
			name: "lock_ttl_is_smaller_than_min",
			lock: &VariableLock{
				TTL:       5 * time.Second,
				LockDelay: minVariableLockTTL - 5*time.Second,
			},
			expErr: errInvalidTTL,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.lock.Validate()
			if !errors.Is(err, tc.expErr) {
				t.Errorf("Expected error %v, but got error %v", tc.expErr, err)
			}
		})
	}
}
