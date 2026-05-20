// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"testing"

	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	regMock "github.com/hashicorp/nomad/client/serviceregistration/mock"
	"github.com/hashicorp/nomad/client/serviceregistration/wrapper"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
	tmock "github.com/stretchr/testify/mock"
)

func TestCheckRestartHook_Prestart(t *testing.T) {
	logger := testlog.HCLogger(t)

	alloc := mock.Alloc()
	alloc.Job.Canonicalize()

	handler := regMock.NewServiceRegistrationHandler(logger)

	mockWatcher := &regMock.MockUniversalWatcher{}
	mockWatcher.On("Watch", tmock.Anything, tmock.Anything, tmock.Anything)
	handler.UniversalWatcher = mockWatcher

	regWrap := wrapper.NewHandlerWrapper(logger, handler, handler)

	service := &structs.Service{
		Provider: "nomad",
		Checks: []*structs.ServiceCheck{
			{
				TaskName: "web",
				CheckRestart: &structs.CheckRestart{
					Limit: 1,
				},
			},
		},
	}

	// group level service
	alloc.Job.LookupTaskGroup("web").Services = []*structs.Service{service}

	// task level service
	task := alloc.LookupTask("web")
	task.Services = []*structs.Service{service}

	testHook := newCheckRestartHook(alloc, task, regWrap, consul.NoopRestarter())

	err := testHook.Prestart(
		t.Context(),
		&interfaces.TaskPrestartRequest{TaskEnv: taskenv.NewEmptyTaskEnv()},
		&interfaces.TaskPrestartResponse{},
	)
	must.NoError(t, err)
	must.True(t, mockWatcher.AssertNumberOfCalls(t, "Watch", 2))
}

func TestCheckRestartHook_Exited(t *testing.T) {
	logger := testlog.HCLogger(t)

	alloc := mock.Alloc()
	alloc.Job.Canonicalize()

	mockWatcher := &regMock.MockUniversalWatcher{}
	mockWatcher.On("Unwatch", tmock.Anything)

	handler := regMock.NewServiceRegistrationHandler(logger)
	handler.UniversalWatcher = mockWatcher

	regWrap := wrapper.NewHandlerWrapper(logger, handler, handler)

	testHook := newCheckRestartHook(alloc, alloc.LookupTask("web"), regWrap, consul.NoopRestarter())
	testHook.checks = []*hookCheck{
		{
			providerType: "nomad",
		},
	}

	err := testHook.Exited(
		t.Context(),
		&interfaces.TaskExitedRequest{},
		&interfaces.TaskExitedResponse{},
	)
	must.NoError(t, err)
	must.True(t, mockWatcher.AssertNumberOfCalls(t, "Unwatch", 1))
}

func TestCheckRestartHook_Stop(t *testing.T) {
	logger := testlog.HCLogger(t)

	alloc := mock.Alloc()
	alloc.Job.Canonicalize()

	mockWatcher := &regMock.MockUniversalWatcher{}
	mockWatcher.On("Unwatch", tmock.Anything)

	handler := regMock.NewServiceRegistrationHandler(logger)
	handler.UniversalWatcher = mockWatcher

	regWrap := wrapper.NewHandlerWrapper(logger, handler, handler)

	testHook := newCheckRestartHook(alloc, alloc.LookupTask("web"), regWrap, consul.NoopRestarter())
	testHook.checks = []*hookCheck{
		{
			providerType: "nomad",
		},
	}

	err := testHook.Stop(
		t.Context(),
		&interfaces.TaskStopRequest{},
		&interfaces.TaskStopResponse{},
	)
	must.NoError(t, err)
	must.True(t, mockWatcher.AssertNumberOfCalls(t, "Unwatch", 1))
}
