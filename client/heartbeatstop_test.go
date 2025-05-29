// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"
)

func TestHeartbeatStop(t *testing.T) {
	ci.Parallel(t)

	shutdownCh := make(chan struct{})
	t.Cleanup(func() { close(shutdownCh) })

	destroyers := map[string]*mockAllocRunnerDestroyer{}

	stopper := newHeartbeatStop(func(id string) (interfaces.AllocRunner, error) {
		return destroyers[id], nil
	},
		time.Hour, // start grace, ignored in this test
		testlog.HCLogger(t),
		shutdownCh)

	// an allocation, with a tiny lease
	alloc1 := &structs.Allocation{
		ID:        uuid.Generate(),
		TaskGroup: "foo",
		Job: &structs.Job{
			TaskGroups: []*structs.TaskGroup{
				{
					Name: "foo",
					Disconnect: &structs.DisconnectStrategy{
						StopOnClientAfter: pointer.Of(time.Microsecond),
					},
				},
			},
		},
	}

	// an alloc with a longer lease
	alloc2 := alloc1.Copy()
	alloc2.ID = uuid.Generate()
	alloc2.Job.TaskGroups[0].Disconnect.StopOnClientAfter = pointer.Of(500 * time.Millisecond)

	// an alloc with no disconnect config
	alloc3 := alloc1.Copy()
	alloc3.ID = uuid.Generate()
	alloc3.Job.TaskGroups[0].Disconnect = nil

	destroyers[alloc1.ID] = &mockAllocRunnerDestroyer{}
	destroyers[alloc2.ID] = &mockAllocRunnerDestroyer{}
	destroyers[alloc3.ID] = &mockAllocRunnerDestroyer{}

	go stopper.watch()
	stopper.allocHook(alloc1)
	stopper.allocHook(alloc2)
	stopper.allocHook(alloc3)

	must.Wait(t, wait.InitialSuccess(
		wait.Timeout(time.Second),
		wait.Gap(10*time.Millisecond),
		wait.ErrorFunc(func() error {
			if destroyers[alloc1.ID].checkCalls() != 1 {
				return errors.New("first alloc was not destroyed as expected")
			}
			if destroyers[alloc2.ID].checkCalls() != 0 {
				return errors.New("second alloc was unexpectedly destroyed")
			}
			if destroyers[alloc3.ID].checkCalls() != 0 {
				return errors.New("third alloc should never be destroyed")
			}
			return nil
		})))

	// send a heartbeat and make sure nothing changes
	stopper.setLastOk(time.Now())

	must.Wait(t, wait.ContinualSuccess(
		wait.Timeout(200*time.Millisecond),
		wait.Gap(10*time.Millisecond),
		wait.ErrorFunc(func() error {
			if destroyers[alloc1.ID].checkCalls() != 1 {
				return errors.New("first alloc should no longer be tracked")
			}
			if destroyers[alloc2.ID].checkCalls() != 0 {
				return errors.New("second alloc was unexpectedly destroyed")
			}
			if destroyers[alloc3.ID].checkCalls() != 0 {
				return errors.New("third alloc should never be destroyed")
			}
			return nil
		})))

	// skip the next heartbeat

	must.Wait(t, wait.InitialSuccess(
		wait.Timeout(1*time.Second),
		wait.Gap(10*time.Millisecond),
		wait.ErrorFunc(func() error {
			if destroyers[alloc1.ID].checkCalls() != 1 {
				return errors.New("first alloc should no longer be tracked")
			}
			if destroyers[alloc2.ID].checkCalls() != 1 {
				return errors.New("second alloc should have been destroyed")
			}
			if destroyers[alloc3.ID].checkCalls() != 0 {
				return errors.New("third alloc should never be destroyed")
			}
			return nil
		})))
}

type mockAllocRunnerDestroyer struct {
	callsLock sync.Mutex
	calls     int
	interfaces.AllocRunner
}

func (ar *mockAllocRunnerDestroyer) checkCalls() int {
	ar.callsLock.Lock()
	defer ar.callsLock.Unlock()
	return ar.calls
}

func (ar *mockAllocRunnerDestroyer) Destroy() {
	ar.callsLock.Lock()
	defer ar.callsLock.Unlock()
	ar.calls++
}
