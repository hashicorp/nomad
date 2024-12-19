// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/users/dynamic"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func TestTaskRunner_DynamicUsersHook_Prestart_unusable(t *testing.T) {
	ci.Parallel(t)

	// task driver does not indicate DynamicWorkloadUsers capability
	const capable = false
	ctx := context.Background()
	logger := testlog.HCLogger(t)

	// if the driver does not indicate the DynamicWorkloadUsers capability,
	// none of the pool, request, or response are touched - so using nil
	// for each of them shows we are exiting the hook immediatly
	var pool dynamic.Pool = nil
	var request *interfaces.TaskPrestartRequest = nil
	var response *interfaces.TaskPrestartResponse = nil

	h := newDynamicUsersHook(ctx, capable, logger, pool)
	must.False(t, h.usable)
	must.NoError(t, h.Prestart(ctx, request, response))
}

func TestTaskRunner_DynamicUsersHook_Prestart_unnecessary(t *testing.T) {
	ci.Parallel(t)

	const capable = true
	ctx := context.Background()
	logger := testlog.HCLogger(t)

	// if the task configures a user, no dynamic workload user will be allocated
	// and we prove this by setting a nil pool
	var pool dynamic.Pool = nil
	var response = new(interfaces.TaskPrestartResponse)
	var request = &interfaces.TaskPrestartRequest{
		Task: &structs.Task{User: "billy"},
	}

	h := newDynamicUsersHook(ctx, capable, logger, pool)
	must.True(t, h.usable)
	must.NoError(t, h.Prestart(ctx, request, response))
	must.MapEmpty(t, response.State)       // no user set
	must.Eq(t, "billy", request.Task.User) // not modified
}

func TestTaskRunner_DynamicUsersHook_Prestart_used(t *testing.T) {
	ci.Parallel(t)

	const capable = true
	ctx := context.Background()
	logger := testlog.HCLogger(t)

	// create a pool allowing UIDs in range [100, 199]
	var pool dynamic.Pool = dynamic.New(&dynamic.PoolConfig{
		MinUGID: 100,
		MaxUGID: 199,
	})
	var response = new(interfaces.TaskPrestartResponse)
	var request = &interfaces.TaskPrestartRequest{
		Task: &structs.Task{User: ""}, // user is not set
	}

	// once the hook runs, check we got an expected ugid and the
	// task user is set to our pseudo dynamic username
	h := newDynamicUsersHook(ctx, capable, logger, pool)
	must.True(t, h.usable)
	must.NoError(t, h.Prestart(ctx, request, response))
	username, exists := response.State[dynamicUsersStateKey]
	must.True(t, exists)
	ugid, err := dynamic.Parse(username)
	must.NoError(t, err)
	must.Between(t, 100, ugid, 199)
	must.Eq(t, username, request.Task.User)
	must.StrHasPrefix(t, "nomad-", username)
}

func TestTaskRunner_DynamicUsersHook_Prestart_exhausted(t *testing.T) {
	ci.Parallel(t)

	const capable = true
	ctx := context.Background()
	logger := testlog.HCLogger(t)

	// create a pool allowing UIDs in range [100, 199]
	var pool dynamic.Pool = dynamic.New(&dynamic.PoolConfig{
		MinUGID: 100,
		MaxUGID: 101,
	})
	pool.Restore(100)
	pool.Restore(101)
	var response = new(interfaces.TaskPrestartResponse)
	var request = &interfaces.TaskPrestartRequest{
		Task: &structs.Task{User: ""}, // user is not set
	}

	h := newDynamicUsersHook(ctx, capable, logger, pool)
	must.True(t, h.usable)
	must.ErrorContains(t, h.Prestart(ctx, request, response), "uid/gid pool exhausted")
}

func TestTaskRunner_DynamicUsersHook_Stop_unusable(t *testing.T) {
	ci.Parallel(t)

	const capable = false
	ctx := context.Background()
	logger := testlog.HCLogger(t)

	// prove we use none of these by setting them all to nil
	var pool dynamic.Pool = nil
	var request *interfaces.TaskStopRequest = nil
	var response *interfaces.TaskStopResponse = nil

	h := newDynamicUsersHook(ctx, capable, logger, pool)
	must.False(t, h.usable)
	must.NoError(t, h.Stop(ctx, request, response))
}

func TestTaskRunner_DynamicUsersHook_Stop_release(t *testing.T) {
	ci.Parallel(t)

	const capable = true
	ctx := context.Background()
	logger := testlog.HCLogger(t)

	// prove we use none of these by setting them all to nil
	var pool dynamic.Pool = dynamic.New(&dynamic.PoolConfig{
		MinUGID: 100,
		MaxUGID: 199,
	})
	pool.Restore(150) // allocate ugid 150
	var request = &interfaces.TaskStopRequest{
		ExistingState: map[string]string{
			dynamicUsersStateKey: "nomad-150",
		},
	}
	var response = new(interfaces.TaskStopResponse)

	h := newDynamicUsersHook(ctx, capable, logger, pool)
	must.True(t, h.usable)
	must.NoError(t, h.Stop(ctx, request, response))
}

func TestTaskRunner_DynamicUsersHook_Stop_malformed(t *testing.T) {
	ci.Parallel(t)

	const capable = true
	ctx := context.Background()
	logger := testlog.HCLogger(t)

	// prove we use none of these by setting them all to nil
	var pool dynamic.Pool = dynamic.New(&dynamic.PoolConfig{
		MinUGID: 100,
		MaxUGID: 199,
	})
	var request = &interfaces.TaskStopRequest{
		ExistingState: map[string]string{
			dynamicUsersStateKey: "not-valid",
		},
	}
	var response = new(interfaces.TaskStopResponse)

	h := newDynamicUsersHook(ctx, capable, logger, pool)
	must.True(t, h.usable)
	must.ErrorContains(t, h.Stop(ctx, request, response), "unable to parse uid/gid from username")
}

func TestTaskRunner_DynamicUsersHook_Stop_not_in_use(t *testing.T) {
	ci.Parallel(t)

	const capable = true
	ctx := context.Background()
	logger := testlog.HCLogger(t)

	// prove we use none of these by setting them all to nil
	var pool dynamic.Pool = dynamic.New(&dynamic.PoolConfig{
		MinUGID: 100,
		MaxUGID: 199,
	})
	var request = &interfaces.TaskStopRequest{
		ExistingState: map[string]string{
			dynamicUsersStateKey: "nomad-101",
		},
	}
	var response = new(interfaces.TaskStopResponse)

	h := newDynamicUsersHook(ctx, capable, logger, pool)
	must.True(t, h.usable)
	must.ErrorContains(t, h.Stop(ctx, request, response), "release of unused uid/gid")
}
