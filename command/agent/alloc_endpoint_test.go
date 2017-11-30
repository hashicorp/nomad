package agent

import (
	"archive/tar"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/golang/snappy"
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/nomad"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/assert"
)

func TestHTTP_AllocsList(t *testing.T) {
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		// Directly manipulate the state
		state := s.Agent.server.State()
		alloc1 := mock.Alloc()
		testEvent := structs.NewTaskEvent(structs.TaskSiblingFailed)
		var events1 []*structs.TaskEvent
		events1 = append(events1, testEvent)
		taskState := &structs.TaskState{Events: events1}
		alloc1.TaskStates = make(map[string]*structs.TaskState)
		alloc1.TaskStates["test"] = taskState

		alloc2 := mock.Alloc()
		alloc2.TaskStates = make(map[string]*structs.TaskState)
		alloc2.TaskStates["test"] = taskState

		state.UpsertJobSummary(998, mock.JobSummary(alloc1.JobID))
		state.UpsertJobSummary(999, mock.JobSummary(alloc2.JobID))
		err := state.UpsertAllocs(1000,
			[]*structs.Allocation{alloc1, alloc2})
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Make the HTTP request
		req, err := http.NewRequest("GET", "/v1/allocations", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.AllocsRequest(respW, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Check for the index
		if respW.HeaderMap.Get("X-Nomad-Index") == "" {
			t.Fatalf("missing index")
		}
		if respW.HeaderMap.Get("X-Nomad-KnownLeader") != "true" {
			t.Fatalf("missing known leader")
		}
		if respW.HeaderMap.Get("X-Nomad-LastContact") == "" {
			t.Fatalf("missing last contact")
		}

		// Check the alloc
		allocs := obj.([]*structs.AllocListStub)
		if len(allocs) != 2 {
			t.Fatalf("bad: %#v", allocs)
		}
		expectedMsg := "Task's sibling failed"
		displayMsg1 := allocs[0].TaskStates["test"].Events[0].DisplayMessage
		assert.Equal(t, expectedMsg, displayMsg1, "DisplayMessage should be set")
		displayMsg2 := allocs[0].TaskStates["test"].Events[0].DisplayMessage
		assert.Equal(t, expectedMsg, displayMsg2, "DisplayMessage should be set")
	})
}

func TestHTTP_AllocsPrefixList(t *testing.T) {
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		// Directly manipulate the state
		state := s.Agent.server.State()

		alloc1 := mock.Alloc()
		alloc1.ID = "aaaaaaaa-e8f7-fd38-c855-ab94ceb89706"
		alloc2 := mock.Alloc()
		alloc2.ID = "aaabbbbb-e8f7-fd38-c855-ab94ceb89706"

		testEvent := structs.NewTaskEvent(structs.TaskSiblingFailed)
		var events1 []*structs.TaskEvent
		events1 = append(events1, testEvent)
		taskState := &structs.TaskState{Events: events1}
		alloc2.TaskStates = make(map[string]*structs.TaskState)
		alloc2.TaskStates["test"] = taskState

		summary1 := mock.JobSummary(alloc1.JobID)
		summary2 := mock.JobSummary(alloc2.JobID)
		if err := state.UpsertJobSummary(998, summary1); err != nil {
			t.Fatal(err)
		}
		if err := state.UpsertJobSummary(999, summary2); err != nil {
			t.Fatal(err)
		}
		if err := state.UpsertAllocs(1000,
			[]*structs.Allocation{alloc1, alloc2}); err != nil {
			t.Fatalf("err: %v", err)
		}

		// Make the HTTP request
		req, err := http.NewRequest("GET", "/v1/allocations?prefix=aaab", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.AllocsRequest(respW, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Check for the index
		if respW.HeaderMap.Get("X-Nomad-Index") == "" {
			t.Fatalf("missing index")
		}
		if respW.HeaderMap.Get("X-Nomad-KnownLeader") != "true" {
			t.Fatalf("missing known leader")
		}
		if respW.HeaderMap.Get("X-Nomad-LastContact") == "" {
			t.Fatalf("missing last contact")
		}

		// Check the alloc
		n := obj.([]*structs.AllocListStub)
		if len(n) != 1 {
			t.Fatalf("bad: %#v", n)
		}

		// Check the identifier
		if n[0].ID != alloc2.ID {
			t.Fatalf("expected alloc ID: %v, Actual: %v", alloc2.ID, n[0].ID)
		}
		expectedMsg := "Task's sibling failed"
		displayMsg1 := n[0].TaskStates["test"].Events[0].DisplayMessage
		assert.Equal(t, expectedMsg, displayMsg1, "DisplayMessage should be set")

	})
}

func TestHTTP_AllocQuery(t *testing.T) {
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		// Directly manipulate the state
		state := s.Agent.server.State()
		alloc := mock.Alloc()
		if err := state.UpsertJobSummary(999, mock.JobSummary(alloc.JobID)); err != nil {
			t.Fatal(err)
		}
		err := state.UpsertAllocs(1000,
			[]*structs.Allocation{alloc})
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Make the HTTP request
		req, err := http.NewRequest("GET", "/v1/allocation/"+alloc.ID, nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.AllocSpecificRequest(respW, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Check for the index
		if respW.HeaderMap.Get("X-Nomad-Index") == "" {
			t.Fatalf("missing index")
		}
		if respW.HeaderMap.Get("X-Nomad-KnownLeader") != "true" {
			t.Fatalf("missing known leader")
		}
		if respW.HeaderMap.Get("X-Nomad-LastContact") == "" {
			t.Fatalf("missing last contact")
		}

		// Check the job
		a := obj.(*structs.Allocation)
		if a.ID != alloc.ID {
			t.Fatalf("bad: %#v", a)
		}
	})
}

func TestHTTP_AllocQuery_Payload(t *testing.T) {
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		// Directly manipulate the state
		state := s.Agent.server.State()
		alloc := mock.Alloc()
		if err := state.UpsertJobSummary(999, mock.JobSummary(alloc.JobID)); err != nil {
			t.Fatal(err)
		}

		// Insert Payload compressed
		expected := []byte("hello world")
		compressed := snappy.Encode(nil, expected)
		alloc.Job.Payload = compressed

		err := state.UpsertAllocs(1000, []*structs.Allocation{alloc})
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Make the HTTP request
		req, err := http.NewRequest("GET", "/v1/allocation/"+alloc.ID, nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		// Make the request
		obj, err := s.Server.AllocSpecificRequest(respW, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Check for the index
		if respW.HeaderMap.Get("X-Nomad-Index") == "" {
			t.Fatalf("missing index")
		}
		if respW.HeaderMap.Get("X-Nomad-KnownLeader") != "true" {
			t.Fatalf("missing known leader")
		}
		if respW.HeaderMap.Get("X-Nomad-LastContact") == "" {
			t.Fatalf("missing last contact")
		}

		// Check the job
		a := obj.(*structs.Allocation)
		if a.ID != alloc.ID {
			t.Fatalf("bad: %#v", a)
		}

		// Check the payload is decompressed
		if !reflect.DeepEqual(a.Job.Payload, expected) {
			t.Fatalf("Payload not decompressed properly; got %#v; want %#v", a.Job.Payload, expected)
		}
	})
}

func TestHTTP_AllocStats(t *testing.T) {
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		// Make the HTTP request
		req, err := http.NewRequest("GET", "/v1/client/allocation/123/foo", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		// Make the request
		_, err = s.Server.ClientAllocRequest(respW, req)
		if !strings.Contains(err.Error(), resourceNotFoundErr) {
			t.Fatalf("err: %v", err)
		}
	})
}

func TestHTTP_AllocStats_ACL(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	httpACLTest(t, nil, func(s *TestAgent) {
		state := s.Agent.server.State()

		// Make the HTTP request
		req, err := http.NewRequest("GET", "/v1/client/allocation/123/stats", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Try request without a token and expect failure
		{
			respW := httptest.NewRecorder()
			_, err := s.Server.ClientAllocRequest(respW, req)
			assert.NotNil(err)
			assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())
		}

		// Try request with an invalid token and expect failure
		{
			respW := httptest.NewRecorder()
			token := mock.CreatePolicyAndToken(t, state, 1005, "invalid", mock.NodePolicy(acl.PolicyWrite))
			setToken(req, token)
			_, err := s.Server.ClientAllocRequest(respW, req)
			assert.NotNil(err)
			assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())
		}

		// Try request with a valid token
		// Still returns an error because the alloc does not exist
		{
			respW := httptest.NewRecorder()
			policy := mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob})
			token := mock.CreatePolicyAndToken(t, state, 1007, "valid", policy)
			setToken(req, token)
			_, err := s.Server.ClientAllocRequest(respW, req)
			assert.NotNil(err)
			assert.Contains(err.Error(), "unknown allocation ID")
		}

		// Try request with a management token
		// Still returns an error because the alloc does not exist
		{
			respW := httptest.NewRecorder()
			setToken(req, s.RootToken)
			_, err := s.Server.ClientAllocRequest(respW, req)
			assert.NotNil(err)
			assert.Contains(err.Error(), "unknown allocation ID")
		}
	})
}

func TestHTTP_AllocSnapshot(t *testing.T) {
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		// Make the HTTP request
		req, err := http.NewRequest("GET", "/v1/client/allocation/123/snapshot", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		// Make the request
		_, err = s.Server.ClientAllocRequest(respW, req)
		if !strings.Contains(err.Error(), allocNotFoundErr) {
			t.Fatalf("err: %v", err)
		}
	})
}

func TestHTTP_AllocSnapshot_WithMigrateToken(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	httpACLTest(t, nil, func(s *TestAgent) {
		// Request without a token fails
		req, err := http.NewRequest("GET", "/v1/client/allocation/123/snapshot", nil)
		assert.Nil(err)

		// Make the unauthorized request
		respW := httptest.NewRecorder()
		_, err = s.Server.ClientAllocRequest(respW, req)
		assert.NotNil(err)
		assert.EqualError(err, structs.ErrPermissionDenied.Error())

		// Create an allocation
		alloc := mock.Alloc()

		validMigrateToken, err := nomad.GenerateMigrateToken(alloc.ID, s.Agent.Client().Node().SecretID)
		assert.Nil(err)

		// Request with a token succeeds
		url := fmt.Sprintf("/v1/client/allocation/%s/snapshot", alloc.ID)
		req, err = http.NewRequest("GET", url, nil)
		assert.Nil(err)

		req.Header.Set("X-Nomad-Token", validMigrateToken)

		// Make the unauthorized request
		respW = httptest.NewRecorder()
		_, err = s.Server.ClientAllocRequest(respW, req)
		assert.NotContains(err.Error(), structs.ErrPermissionDenied.Error())
	})
}

// TestHTTP_AllocSnapshot_Atomic ensures that when a client encounters an error
// snapshotting a valid tar is not returned.
func TestHTTP_AllocSnapshot_Atomic(t *testing.T) {
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		// Create an alloc
		state := s.server.State()
		alloc := mock.Alloc()
		alloc.Job.TaskGroups[0].Tasks[0].Driver = "mock_driver"
		alloc.Job.TaskGroups[0].Tasks[0].Config["run_for"] = "30s"
		alloc.NodeID = s.client.NodeID()
		state.UpsertJobSummary(998, mock.JobSummary(alloc.JobID))
		if err := state.UpsertAllocs(1000, []*structs.Allocation{alloc.Copy()}); err != nil {
			t.Fatalf("error upserting alloc: %v", err)
		}

		// Wait for the client to run it
		testutil.WaitForResult(func() (bool, error) {
			if _, err := s.client.GetClientAlloc(alloc.ID); err != nil {
				return false, err
			}

			serverAlloc, err := state.AllocByID(nil, alloc.ID)
			if err != nil {
				return false, err
			}

			return serverAlloc.ClientStatus == structs.AllocClientStatusRunning, fmt.Errorf(serverAlloc.ClientStatus)
		}, func(err error) {
			t.Fatalf("client not running alloc: %v", err)
		})

		// Now write to its shared dir
		allocDirI, err := s.client.GetAllocFS(alloc.ID)
		if err != nil {
			t.Fatalf("unable to find alloc dir: %v", err)
		}
		allocDir := allocDirI.(*allocdir.AllocDir)

		// Remove the task dir to break Snapshot
		os.RemoveAll(allocDir.TaskDirs["web"].LocalDir)

		// Assert Snapshot fails
		if err := allocDir.Snapshot(ioutil.Discard); err != nil {
			s.logger.Printf("[DEBUG] agent.test: snapshot returned error: %v", err)
		} else {
			t.Errorf("expected Snapshot() to fail but it did not")
		}

		// Make the HTTP request to ensure the Snapshot error is
		// propagated through to the HTTP layer. Since the tar is
		// streamed over a 200 HTTP response the only way to signal an
		// error is by writing a marker file.
		respW := httptest.NewRecorder()
		req, err := http.NewRequest("GET", fmt.Sprintf("/v1/client/allocation/%s/snapshot", alloc.ID), nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Make the request via the mux to make sure the error returned
		// by Snapshot is properly propogated via HTTP
		s.Server.mux.ServeHTTP(respW, req)
		resp := respW.Result()
		r := tar.NewReader(resp.Body)
		errorFilename := allocdir.SnapshotErrorFilename(alloc.ID)
		markerFound := false
		markerContents := ""
		for {
			header, err := r.Next()
			if err != nil {
				if err != io.EOF {
					// Huh, I wonder how a non-EOF error can happen?
					t.Errorf("Unexpected error while streaming: %v", err)
				}
				break
			}

			if markerFound {
				// No more files should be found after the failure marker
				t.Errorf("Next file found after error marker: %s", header.Name)
				break
			}

			if header.Name == errorFilename {
				// Found it!
				markerFound = true
				buf := make([]byte, int(header.Size))
				if _, err := r.Read(buf); err != nil {
					t.Errorf("Unexpected error reading error marker %s: %v", errorFilename, err)
				} else {
					markerContents = string(buf)
				}
			}
		}

		if !markerFound {
			t.Fatalf("marker file %s not written; bad tar will be treated as good!", errorFilename)
		}
		if markerContents == "" {
			t.Fatalf("marker file %s empty", markerContents)
		} else {
			t.Logf("EXPECTED snapshot error: %s", markerContents)
		}
	})
}

func TestHTTP_AllocGC(t *testing.T) {
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		// Make the HTTP request
		req, err := http.NewRequest("GET", "/v1/client/allocation/123/gc", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		// Make the request
		_, err = s.Server.ClientAllocRequest(respW, req)
		if !strings.Contains(err.Error(), "unable to collect allocation") {
			t.Fatalf("err: %v", err)
		}
	})
}

func TestHTTP_AllocGC_ACL(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	httpACLTest(t, nil, func(s *TestAgent) {
		state := s.Agent.server.State()

		// Make the HTTP request
		req, err := http.NewRequest("GET", "/v1/client/allocation/123/gc", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Try request without a token and expect failure
		{
			respW := httptest.NewRecorder()
			_, err := s.Server.ClientAllocRequest(respW, req)
			assert.NotNil(err)
			assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())
		}

		// Try request with an invalid token and expect failure
		{
			respW := httptest.NewRecorder()
			token := mock.CreatePolicyAndToken(t, state, 1005, "invalid", mock.NodePolicy(acl.PolicyWrite))
			setToken(req, token)
			_, err := s.Server.ClientAllocRequest(respW, req)
			assert.NotNil(err)
			assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())
		}

		// Try request with a valid token
		// Still returns an error because the alloc does not exist
		{
			respW := httptest.NewRecorder()
			policy := mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilitySubmitJob})
			token := mock.CreatePolicyAndToken(t, state, 1007, "valid", policy)
			setToken(req, token)
			_, err := s.Server.ClientAllocRequest(respW, req)
			assert.NotNil(err)
			assert.Contains(err.Error(), "not present")
		}

		// Try request with a management token
		// Still returns an error because the alloc does not exist
		{
			respW := httptest.NewRecorder()
			setToken(req, s.RootToken)
			_, err := s.Server.ClientAllocRequest(respW, req)
			assert.NotNil(err)
			assert.Contains(err.Error(), "not present")
		}
	})
}

func TestHTTP_AllocAllGC(t *testing.T) {
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		// Make the HTTP request
		req, err := http.NewRequest("GET", "/v1/client/gc", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		// Make the request
		_, err = s.Server.ClientGCRequest(respW, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	})

}

func TestHTTP_AllocAllGC_ACL(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	httpACLTest(t, nil, func(s *TestAgent) {
		state := s.Agent.server.State()

		// Make the HTTP request
		req, err := http.NewRequest("GET", "/v1/client/gc", nil)
		assert.Nil(err)

		// Try request without a token and expect failure
		{
			respW := httptest.NewRecorder()
			_, err := s.Server.ClientGCRequest(respW, req)
			assert.NotNil(err)
			assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())
		}

		// Try request with an invalid token and expect failure
		{
			respW := httptest.NewRecorder()
			token := mock.CreatePolicyAndToken(t, state, 1005, "invalid", mock.NodePolicy(acl.PolicyRead))
			setToken(req, token)
			_, err := s.Server.ClientGCRequest(respW, req)
			assert.NotNil(err)
			assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())
		}

		// Try request with a valid token
		{
			respW := httptest.NewRecorder()
			token := mock.CreatePolicyAndToken(t, state, 1007, "valid", mock.NodePolicy(acl.PolicyWrite))
			setToken(req, token)
			_, err := s.Server.ClientGCRequest(respW, req)
			assert.Nil(err)
			assert.Equal(http.StatusOK, respW.Code)
		}

		// Try request with a management token
		{
			respW := httptest.NewRecorder()
			setToken(req, s.RootToken)
			_, err := s.Server.ClientGCRequest(respW, req)
			assert.Nil(err)
			assert.Equal(http.StatusOK, respW.Code)
		}
	})

}
