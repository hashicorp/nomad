package agent

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/assert"
)

func TestHTTP_ResourcesWithIllegalMethod(t *testing.T) {
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		req, err := http.NewRequest("DELETE", "/v1/resources", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		_, err = s.Server.ResourcesRequest(respW, req)
		assert.NotNil(t, err, "HTTP DELETE should not be accepted for this endpoint")
	})
}

func createJobForTest(jobID string, s *TestAgent, t *testing.T) {
	job := mock.Job()
	job.ID = jobID
	job.TaskGroups[0].Count = 1

	state := s.Agent.server.State()
	err := state.UpsertJob(1000, job)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestHTTP_Resources_SingleJob(t *testing.T) {
	testJob := "aaaaaaaa-e8f7-fd38-c855-ab94ceb89706"
	testJobPrefix := "aaaaaaaa-e8f7-fd38"
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		createJobForTest(testJob, s, t)

		data := structs.ResourcesRequest{Prefix: testJobPrefix, Context: "job"}
		req, err := http.NewRequest("POST", "/v1/resources", encodeReq(data))

		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		resp, err := s.Server.ResourcesRequest(respW, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		res := resp.(structs.ResourcesResponse)
		if len(res.Matches) != 1 {
			t.Fatalf("No expected key values in resources list")
		}

		j := res.Matches["job"]
		if j == nil || len(j) != 1 {
			t.Fatalf("The number of jobs that were returned does not equal the number of jobs we expected (1)", j)
		}

		assert.Equal(t, j[0], testJob)
		assert.Equal(t, res.Truncations["job"], false)
	})
}

func TestHTTP_Resources_MultipleJobs(t *testing.T) {
	testJobA := "aaaaaaaa-e8f7-fd38-c855-ab94ceb89706"
	testJobB := "aaaaaaaa-e8f7-fd38-c855-ab94ceb89707"
	testJobC := "bbbbbbbb-e8f7-fd38-c855-ab94ceb89707"

	testJobPrefix := "aaaaaaaa-e8f7-fd38"

	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		createJobForTest(testJobA, s, t)
		createJobForTest(testJobB, s, t)
		createJobForTest(testJobC, s, t)

		data := structs.ResourcesRequest{Prefix: testJobPrefix, Context: "job"}
		req, err := http.NewRequest("POST", "/v1/resources", encodeReq(data))

		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		resp, err := s.Server.ResourcesRequest(respW, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		res := resp.(structs.ResourcesResponse)
		if len(res.Matches) != 1 {
			t.Fatalf("No expected key values in resources list")
		}

		j := res.Matches["job"]
		if j == nil || len(j) != 2 {
			t.Fatalf("The number of jobs that were returned does not equal the number of jobs we expected (2)", j)
		}

		assert.Contains(t, j, testJobA)
		assert.Contains(t, j, testJobB)
		assert.NotContains(t, j, testJobC)
	})
}

func TestHTTP_ResoucesList_Evaluation(t *testing.T) {
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		state := s.Agent.server.State()
		eval1 := mock.Eval()
		eval2 := mock.Eval()
		err := state.UpsertEvals(1000,
			[]*structs.Evaluation{eval1, eval2})
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Make the HTTP request
		evalPrefix := eval1.ID[:len(eval1.ID)-2]
		data := structs.ResourcesRequest{Prefix: evalPrefix, Context: "eval"}
		req, err := http.NewRequest("POST", "/v1/resources", encodeReq(data))
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		resp, err := s.Server.ResourcesRequest(respW, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		res := resp.(structs.ResourcesResponse)
		if len(res.Matches) != 1 {
			t.Fatalf("No expected key values in resources list")
		}

		j := res.Matches["eval"]
		if len(j) != 1 {
			t.Fatalf("The number of evaluations that were returned does not equal the number we expected (1)", j)
		}

		assert.Contains(t, j, eval1.ID)
		assert.NotContains(t, j, eval2.ID)
		assert.Equal(t, res.Truncations["eval"], false)
	})
}

// TODO
//func TestHTTP_ResourcesWithNoJob(t *testing.T) {
//}
