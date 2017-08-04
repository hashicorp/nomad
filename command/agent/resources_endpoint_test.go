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

func TestHTTP_Resources_POST(t *testing.T) {
	testJob := "aaaaaaaa-e8f7-fd38-c855-ab94ceb89706"
	testJobPrefix := "aaaaaaaa-e8f7-fd38"
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		createJobForTest(testJob, s, t)

		data := structs.ResourcesRequest{Prefix: testJobPrefix, Context: "jobs"}
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

		assert.Equal(t, 1, len(res.Matches))

		j := res.Matches["jobs"]

		assert.Equal(t, 1, len(j))
		assert.Equal(t, j[0], testJob)

		assert.Equal(t, res.Truncations["job"], false)
		assert.NotEqual(t, "0", respW.HeaderMap.Get("X-Nomad-Index"))
	})
}

func TestHTTP_Resources_PUT(t *testing.T) {
	testJob := "aaaaaaaa-e8f7-fd38-c855-ab94ceb89706"
	testJobPrefix := "aaaaaaaa-e8f7-fd38"
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		createJobForTest(testJob, s, t)

		data := structs.ResourcesRequest{Prefix: testJobPrefix, Context: "jobs"}
		req, err := http.NewRequest("PUT", "/v1/resources", encodeReq(data))

		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		resp, err := s.Server.ResourcesRequest(respW, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		res := resp.(structs.ResourcesResponse)

		assert.Equal(t, 1, len(res.Matches))

		j := res.Matches["jobs"]

		assert.Equal(t, 1, len(j))
		assert.Equal(t, j[0], testJob)

		assert.Equal(t, res.Truncations["job"], false)
		assert.NotEqual(t, "0", respW.HeaderMap.Get("X-Nomad-Index"))
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

		data := structs.ResourcesRequest{Prefix: testJobPrefix, Context: "jobs"}
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

		assert.Equal(t, 1, len(res.Matches))

		j := res.Matches["jobs"]

		assert.Equal(t, 2, len(j))
		assert.Contains(t, j, testJobA)
		assert.Contains(t, j, testJobB)
		assert.NotContains(t, j, testJobC)

		assert.Equal(t, res.Truncations["job"], false)
		assert.NotEqual(t, "0", respW.HeaderMap.Get("X-Nomad-Index"))
	})
}

func TestHTTP_ResoucesList_Evaluation(t *testing.T) {
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		state := s.Agent.server.State()
		eval1 := mock.Eval()
		eval2 := mock.Eval()
		err := state.UpsertEvals(9000,
			[]*structs.Evaluation{eval1, eval2})
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		prefix := eval1.ID[:len(eval1.ID)-2]
		data := structs.ResourcesRequest{Prefix: prefix, Context: "evals"}
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

		assert.Equal(t, 1, len(res.Matches))

		j := res.Matches["evals"]
		assert.Equal(t, 1, len(j))
		assert.Contains(t, j, eval1.ID)
		assert.NotContains(t, j, eval2.ID)

		assert.Equal(t, res.Truncations["evals"], false)
		assert.Equal(t, "9000", respW.HeaderMap.Get("X-Nomad-Index"))
	})
}

func TestHTTP_Resources_NoJob(t *testing.T) {
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		data := structs.ResourcesRequest{Prefix: "12345", Context: "jobs"}
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

		assert.Equal(t, 1, len(res.Matches))
		assert.Equal(t, 0, len(res.Matches["jobs"]))

		assert.Equal(t, "0", respW.HeaderMap.Get("X-Nomad-Index"))
	})
}

func TestHTTP_Resources_NoContext(t *testing.T) {
	testJobID := "aaaaaaaa-e8f7-fd38-c855-ab94ceb89706"
	testJobPrefix := "aaaaaaaa-e8f7-fd38"
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		createJobForTest(testJobID, s, t)

		state := s.Agent.server.State()
		eval1 := mock.Eval()
		eval1.ID = testJobID
		err := state.UpsertEvals(9000,
			[]*structs.Evaluation{eval1})
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		data := structs.ResourcesRequest{Prefix: testJobPrefix}
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

		matchedJobs := res.Matches["jobs"]
		matchedEvals := res.Matches["evals"]

		assert.Equal(t, 1, len(matchedJobs))
		assert.Equal(t, 1, len(matchedEvals))

		assert.Equal(t, matchedJobs[0], testJobID)
		assert.Equal(t, matchedEvals[0], eval1.ID)
	})
}
