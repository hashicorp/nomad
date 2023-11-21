// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"errors"
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
)

func TestAction_Copy(t *testing.T) {
	ci.Parallel(t)

	var inputAction *Action
	must.Nil(t, inputAction.Copy())

	inputAction = &Action{
		Name:    "adrian-iv",
		Command: "pope",
		Args:    []string{"1154", "1159"},
	}

	actionCopy := inputAction.Copy()
	must.Equal(t, inputAction, actionCopy)
	must.NotEq(t, fmt.Sprint(&inputAction), fmt.Sprint(&actionCopy))
}

func TestAction_Equal(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name            string
		inputAction     *Action
		inputFuncAction *Action
		expectedOutput  bool
	}{
		{
			name:            "both nil",
			inputAction:     nil,
			inputFuncAction: nil,
			expectedOutput:  true,
		},
		{
			name:            "a nil",
			inputAction:     nil,
			inputFuncAction: &Action{},
			expectedOutput:  false,
		},
		{
			name:            "o nil",
			inputAction:     &Action{},
			inputFuncAction: nil,
			expectedOutput:  false,
		},
		{
			name: "name changed",
			inputAction: &Action{
				Name:    "original-action",
				Command: "env",
				Args:    []string{"foo", "bar"},
			},
			inputFuncAction: &Action{
				Name:    "original-action-ng",
				Command: "env",
				Args:    []string{"foo", "bar"},
			},
			expectedOutput: false,
		},
		{
			name: "args changed",
			inputAction: &Action{
				Name:    "original-action",
				Command: "env",
				Args:    []string{"foo", "bar"},
			},
			inputFuncAction: &Action{
				Name:    "original-action",
				Command: "env",
				Args:    []string{"foo", "bar", "baz"},
			},
			expectedOutput: false,
		},
		{
			name: "command changed",
			inputAction: &Action{
				Name:    "original-action",
				Command: "env",
				Args:    []string{"foo", "bar"},
			},
			inputFuncAction: &Action{
				Name:    "original-action",
				Command: "go env",
				Args:    []string{"foo", "bar"},
			},
			expectedOutput: false,
		},
		{
			name: "full equal",
			inputAction: &Action{
				Name:    "original-action",
				Command: "env",
				Args:    []string{"foo", "bar"},
			},
			inputFuncAction: &Action{
				Name:    "original-action",
				Command: "env",
				Args:    []string{"foo", "bar"},
			},
			expectedOutput: true,
		},
		{
			name: "partial equal",
			inputAction: &Action{
				Name:    "original-action",
				Command: "env",
				Args:    []string{},
			},
			inputFuncAction: &Action{
				Name:    "original-action",
				Command: "env",
				Args:    []string{},
			},
			expectedOutput: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			must.Eq(t, tc.expectedOutput, tc.inputAction.Equal(tc.inputFuncAction))
		})
	}
}

func TestAction_Validate(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name          string
		inputAction   *Action
		expectedError error
	}{
		{
			name:          "nil",
			inputAction:   nil,
			expectedError: nil,
		},
		{
			name:          "empty command",
			inputAction:   &Action{},
			expectedError: errors.New("Missing command"),
		},
		{
			name:          "valid",
			inputAction:   &Action{Command: "env"},
			expectedError: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualError := tc.inputAction.Validate()
			if tc.expectedError != nil {
				must.ErrorContains(t, actualError, tc.expectedError.Error())
			} else {
				must.NoError(t, actualError)
			}
		})
	}
}
