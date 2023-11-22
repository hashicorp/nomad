// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

// Actions are executable commands that can be run on an allocation within
// the context of a task. They are left open-ended enough to be applied to
// other Nomad concepts like Nodes in the future.

package structs

import (
	"errors"
	"fmt"
	"regexp"
	"slices"

	"github.com/hashicorp/go-multierror"
)

// validJobActionName is used to validate a job action name.
var validJobActionName = regexp.MustCompile("^[a-zA-Z0-9-]{1,128}$")

type Action struct {
	Name    string
	Command string
	Args    []string
}

type JobAction struct {
	Action
	TaskName      string
	TaskGroupName string
}

const (
	// JobGetActionsRPCMethod is the RPC method for listing all configured
	// actions within a job.
	//
	// Args: JobActionListRequest
	// Reply: JobActionListResponse
	JobGetActionsRPCMethod = "Job.GetActions"
)

// JobActionListRequest is the request object when listing the actions
// configured within a job.
type JobActionListRequest struct {
	JobID string
	QueryOptions
}

// JobActionListResponse is the response object when performing a listing of
// actions configured within a job.
type JobActionListResponse struct {
	Actions []*JobAction
	QueryMeta
}

func (a *Action) Copy() *Action {
	if a == nil {
		return nil
	}
	na := new(Action)
	*na = *a
	na.Args = slices.Clone(a.Args)
	return na
}

func (a *Action) Equal(o *Action) bool {
	if a == nil && o == nil {
		return true
	}
	if a == nil || o == nil {
		return false
	}
	return a.Name == o.Name &&
		a.Command == o.Command &&
		slices.Equal(a.Args, o.Args)
}

func (a *Action) Validate() error {
	if a == nil {
		return nil
	}

	var mErr *multierror.Error
	if a.Command == "" {
		mErr = multierror.Append(mErr, errors.New("command cannot be empty"))
	}
	if !validJobActionName.MatchString(a.Name) {
		mErr = multierror.Append(mErr, fmt.Errorf("invalid name '%s'", a.Name))
	}

	return mErr.ErrorOrNil()
}
