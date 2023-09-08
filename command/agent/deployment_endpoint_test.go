// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/assert"
)

func TestHTTP_DeploymentList(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)
	httpTest(t, nil, func(s *TestAgent) {
		// Directly manipulate the state
		state := s.Agent.server.State()
		d1 := mock.Deployment()
		d2 := mock.Deployment()
		assert.Nil(state.UpsertDeployment(999, d1), "UpsertDeployment")
		assert.Nil(state.UpsertDeployment(1000, d2), "UpsertDeployment")

		// Make the HTTP request
		req, err := http.NewRequest(http.MethodGet, "/v1/deployments", nil)
		assert.Nil(err, "HTTP Request")
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.DeploymentsRequest(respW, req)
		assert.Nil(err, "Deployment Request")

		// Check for the index
		assert.NotZero(respW.Result().Header.Get("X-Nomad-Index"), "missing index")
		assert.Equal("true", respW.Result().Header.Get("X-Nomad-KnownLeader"), "missing known leader")
		assert.NotZero(respW.Result().Header.Get("X-Nomad-LastContact"), "missing last contact")

		// Check the deployments
		deploys := obj.([]*structs.Deployment)
		assert.Len(deploys, 2, "Deployments")
	})
}

func TestHTTP_DeploymentPrefixList(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)
	httpTest(t, nil, func(s *TestAgent) {
		// Directly manipulate the state
		state := s.Agent.server.State()
		d1 := mock.Deployment()
		d1.ID = "aaabbbbb-e8f7-fd38-c855-ab94ceb89706"
		d2 := mock.Deployment()
		d2.ID = "aaabbbbb-e8f7-fd38-c855-ab94ceb89706"
		assert.Nil(state.UpsertDeployment(999, d1), "UpsertDeployment")
		assert.Nil(state.UpsertDeployment(1000, d2), "UpsertDeployment")

		// Make the HTTP request
		req, err := http.NewRequest(http.MethodGet, "/v1/deployments?prefix=aaab", nil)
		assert.Nil(err, "HTTP Request")
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.DeploymentsRequest(respW, req)
		assert.Nil(err, "Deployment Request")

		// Check for the index
		assert.NotZero(respW.Result().Header.Get("X-Nomad-Index"), "missing index")
		assert.Equal("true", respW.Result().Header.Get("X-Nomad-KnownLeader"), "missing known leader")
		assert.NotZero(respW.Result().Header.Get("X-Nomad-LastContact"), "missing last contact")

		// Check the deployments
		deploys := obj.([]*structs.Deployment)
		assert.Len(deploys, 1, "Deployments")
		assert.Equal(d1.ID, deploys[0].ID, "Wrong Deployment")
	})
}

func TestHTTP_DeploymentAllocations(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)
	httpTest(t, nil, func(s *TestAgent) {
		// Directly manipulate the state
		state := s.Agent.server.State()
		j := mock.Job()
		d := mock.Deployment()
		d.JobID = j.ID
		a1 := mock.Alloc()
		a1.JobID = j.ID
		a1.DeploymentID = d.ID

		testEvent := structs.NewTaskEvent(structs.TaskSiblingFailed)
		var events1 []*structs.TaskEvent
		events1 = append(events1, testEvent)
		taskState := &structs.TaskState{Events: events1}
		a1.TaskStates = make(map[string]*structs.TaskState)
		a1.TaskStates["test"] = taskState

		a2 := mock.Alloc()
		a2.JobID = j.ID
		a2.DeploymentID = d.ID

		// Create a test event
		testEvent2 := structs.NewTaskEvent(structs.TaskSiblingFailed)
		var events2 []*structs.TaskEvent
		events2 = append(events2, testEvent2)
		taskState2 := &structs.TaskState{Events: events2}
		a2.TaskStates = make(map[string]*structs.TaskState)
		a2.TaskStates["test"] = taskState2

		assert.Nil(state.UpsertJob(structs.MsgTypeTestSetup, 998, nil, j), "UpsertJob")
		assert.Nil(state.UpsertDeployment(999, d), "UpsertDeployment")
		assert.Nil(state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{a1, a2}), "UpsertAllocs")

		// Make the HTTP request
		req, err := http.NewRequest(http.MethodGet, "/v1/deployment/allocations/"+d.ID, nil)
		assert.Nil(err, "HTTP Request")
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.DeploymentSpecificRequest(respW, req)
		assert.Nil(err, "DeploymentSpecificRequest")

		// Check for the index
		assert.NotZero(respW.Result().Header.Get("X-Nomad-Index"), "missing index")
		assert.Equal("true", respW.Result().Header.Get("X-Nomad-KnownLeader"), "missing known leader")
		assert.NotZero(respW.Result().Header.Get("X-Nomad-LastContact"), "missing last contact")

		// Check the output
		allocs := obj.([]*structs.AllocListStub)
		assert.Len(allocs, 2, "Deployment Allocs")
		expectedMsg := "Task's sibling failed"
		displayMsg1 := allocs[0].TaskStates["test"].Events[0].DisplayMessage
		assert.Equal(expectedMsg, displayMsg1, "DisplayMessage should be set")
		displayMsg2 := allocs[0].TaskStates["test"].Events[0].DisplayMessage
		assert.Equal(expectedMsg, displayMsg2, "DisplayMessage should be set")
	})
}

func TestHTTP_DeploymentQuery(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)
	httpTest(t, nil, func(s *TestAgent) {
		// Directly manipulate the state
		state := s.Agent.server.State()
		d := mock.Deployment()
		assert.Nil(state.UpsertDeployment(1000, d), "UpsertDeployment")

		// Make the HTTP request
		req, err := http.NewRequest(http.MethodGet, "/v1/deployment/"+d.ID, nil)
		assert.Nil(err, "HTTP Request")
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.DeploymentSpecificRequest(respW, req)
		assert.Nil(err, "Deployment Request")

		// Check for the index
		assert.NotZero(respW.Result().Header.Get("X-Nomad-Index"), "missing index")
		assert.Equal("true", respW.Result().Header.Get("X-Nomad-KnownLeader"), "missing known leader")
		assert.NotZero(respW.Result().Header.Get("X-Nomad-LastContact"), "missing last contact")

		// Check the job
		out := obj.(*structs.Deployment)
		assert.Equal(d.ID, out.ID, "ID mismatch")
	})
}

func TestHTTP_DeploymentPause(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)
	httpTest(t, nil, func(s *TestAgent) {
		// Directly manipulate the state
		state := s.Agent.server.State()
		j := mock.Job()
		d := mock.Deployment()
		d.JobID = j.ID
		assert.Nil(state.UpsertJob(structs.MsgTypeTestSetup, 999, nil, j), "UpsertJob")
		assert.Nil(state.UpsertDeployment(1000, d), "UpsertDeployment")

		// Create the pause request
		args := structs.DeploymentPauseRequest{
			DeploymentID: d.ID,
			Pause:        false,
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				Namespace: structs.DefaultNamespace,
			},
		}
		buf := encodeReq(args)

		// Make the HTTP request
		req, err := http.NewRequest(http.MethodPut, "/v1/deployment/pause/"+d.ID, buf)
		assert.Nil(err, "HTTP Request")
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.DeploymentSpecificRequest(respW, req)
		assert.Nil(err, "Deployment Request")

		// Check the response
		resp := obj.(structs.DeploymentUpdateResponse)
		assert.NotZero(resp.EvalID, "Expect Eval")
		assert.NotZero(resp.EvalCreateIndex, "Expect Eval")
		assert.NotZero(resp.DeploymentModifyIndex, "Expect Deployment to be Modified")
		assert.NotZero(respW.Result().Header.Get("X-Nomad-Index"), "missing index")
	})
}

func TestHTTP_DeploymentPromote(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)
	httpTest(t, nil, func(s *TestAgent) {
		// Directly manipulate the state
		state := s.Agent.server.State()
		j := mock.Job()
		d := mock.Deployment()
		d.JobID = j.ID
		assert.Nil(state.UpsertJob(structs.MsgTypeTestSetup, 999, nil, j), "UpsertJob")
		assert.Nil(state.UpsertDeployment(1000, d), "UpsertDeployment")

		// Create the pause request
		args := structs.DeploymentPromoteRequest{
			DeploymentID: d.ID,
			All:          true,
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				Namespace: structs.DefaultNamespace,
			},
		}
		buf := encodeReq(args)

		// Make the HTTP request
		req, err := http.NewRequest(http.MethodPut, "/v1/deployment/pause/"+d.ID, buf)
		assert.Nil(err, "HTTP Request")
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.DeploymentSpecificRequest(respW, req)
		assert.Nil(err, "Deployment Request")

		// Check the response
		resp := obj.(structs.DeploymentUpdateResponse)
		assert.NotZero(resp.EvalID, "Expect Eval")
		assert.NotZero(resp.EvalCreateIndex, "Expect Eval")
		assert.NotZero(resp.DeploymentModifyIndex, "Expect Deployment to be Modified")
		assert.NotZero(respW.Result().Header.Get("X-Nomad-Index"), "missing index")
	})
}

func TestHTTP_DeploymentAllocHealth(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)
	httpTest(t, nil, func(s *TestAgent) {
		// Directly manipulate the state
		state := s.Agent.server.State()
		j := mock.Job()
		d := mock.Deployment()
		d.JobID = j.ID
		a := mock.Alloc()
		a.JobID = j.ID
		a.DeploymentID = d.ID
		assert.Nil(state.UpsertJob(structs.MsgTypeTestSetup, 998, nil, j), "UpsertJob")
		assert.Nil(state.UpsertDeployment(999, d), "UpsertDeployment")
		assert.Nil(state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{a}), "UpsertAllocs")

		// Create the pause request
		args := structs.DeploymentAllocHealthRequest{
			DeploymentID:         d.ID,
			HealthyAllocationIDs: []string{a.ID},
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				Namespace: structs.DefaultNamespace,
			},
		}
		buf := encodeReq(args)

		// Make the HTTP request
		req, err := http.NewRequest(http.MethodPut, "/v1/deployment/allocation-health/"+d.ID, buf)
		assert.Nil(err, "HTTP Request")
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.DeploymentSpecificRequest(respW, req)
		assert.Nil(err, "Deployment Request")

		// Check the response
		resp := obj.(structs.DeploymentUpdateResponse)
		assert.NotZero(resp.EvalID, "Expect Eval")
		assert.NotZero(resp.EvalCreateIndex, "Expect Eval")
		assert.NotZero(resp.DeploymentModifyIndex, "Expect Deployment to be Modified")
		assert.NotZero(respW.Result().Header.Get("X-Nomad-Index"), "missing index")
	})
}

func TestHTTP_DeploymentFail(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)
	httpTest(t, nil, func(s *TestAgent) {
		// Directly manipulate the state
		state := s.Agent.server.State()
		j := mock.Job()
		d := mock.Deployment()
		d.JobID = j.ID
		assert.Nil(state.UpsertJob(structs.MsgTypeTestSetup, 998, nil, j), "UpsertJob")
		assert.Nil(state.UpsertDeployment(999, d), "UpsertDeployment")

		// Make the HTTP request
		req, err := http.NewRequest(http.MethodPut, "/v1/deployment/fail/"+d.ID, nil)
		assert.Nil(err, "HTTP Request")
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.DeploymentSpecificRequest(respW, req)
		assert.Nil(err, "Deployment Request")

		// Check the response
		resp := obj.(structs.DeploymentUpdateResponse)
		assert.NotZero(resp.EvalID, "Expect Eval")
		assert.NotZero(resp.EvalCreateIndex, "Expect Eval")
		assert.NotZero(resp.DeploymentModifyIndex, "Expect Deployment to be Modified")
		assert.NotZero(respW.Result().Header.Get("X-Nomad-Index"), "missing index")
	})
}
