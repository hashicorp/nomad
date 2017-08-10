package agent

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	a "github.com/stretchr/testify/assert"
)

func TestHTTP_SearchWithIllegalMethod(t *testing.T) {
	assert := a.New(t)
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		req, err := http.NewRequest("DELETE", "/v1/resources", nil)
		assert.Nil(err)
		respW := httptest.NewRecorder()

		_, err = s.Server.SearchRequest(respW, req)
		assert.NotNil(err, "HTTP DELETE should not be accepted for this endpoint")
	})
}

func createJobForTest(jobID string, s *TestAgent, t *testing.T) {
	assert := a.New(t)

	job := mock.Job()
	job.ID = jobID
	job.TaskGroups[0].Count = 1

	state := s.Agent.server.State()
	err := state.UpsertJob(1000, job)
	assert.Nil(err)
}

func TestHTTP_Search_POST(t *testing.T) {
	assert := a.New(t)

	testJob := "aaaaaaaa-e8f7-fd38-c855-ab94ceb89706"
	testJobPrefix := "aaaaaaaa-e8f7-fd38"
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		createJobForTest(testJob, s, t)

		data := structs.SearchRequest{Prefix: testJobPrefix, Context: "jobs"}
		req, err := http.NewRequest("POST", "/v1/search", encodeReq(data))
		assert.Nil(err)

		respW := httptest.NewRecorder()

		resp, err := s.Server.SearchRequest(respW, req)
		assert.Nil(err)

		res := resp.(structs.SearchResponse)

		assert.Equal(1, len(res.Matches))

		j := res.Matches["jobs"]

		assert.Equal(1, len(j))
		assert.Equal(j[0], testJob)

		assert.Equal(res.Truncations["job"], false)
		assert.NotEqual("0", respW.HeaderMap.Get("X-Nomad-Index"))
	})
}

func TestHTTP_Search_PUT(t *testing.T) {
	assert := a.New(t)

	testJob := "aaaaaaaa-e8f7-fd38-c855-ab94ceb89706"
	testJobPrefix := "aaaaaaaa-e8f7-fd38"
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		createJobForTest(testJob, s, t)

		data := structs.SearchRequest{Prefix: testJobPrefix, Context: "jobs"}
		req, err := http.NewRequest("PUT", "/v1/search", encodeReq(data))
		assert.Nil(err)

		respW := httptest.NewRecorder()

		resp, err := s.Server.SearchRequest(respW, req)
		assert.Nil(err)

		res := resp.(structs.SearchResponse)

		assert.Equal(1, len(res.Matches))

		j := res.Matches["jobs"]

		assert.Equal(1, len(j))
		assert.Equal(j[0], testJob)

		assert.Equal(res.Truncations["job"], false)
		assert.NotEqual("0", respW.HeaderMap.Get("X-Nomad-Index"))
	})
}

func TestHTTP_Search_MultipleJobs(t *testing.T) {
	assert := a.New(t)

	testJobA := "aaaaaaaa-e8f7-fd38-c855-ab94ceb89706"
	testJobB := "aaaaaaaa-e8f7-fd38-c855-ab94ceb89707"
	testJobC := "bbbbbbbb-e8f7-fd38-c855-ab94ceb89707"

	testJobPrefix := "aaaaaaaa-e8f7-fd38"

	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		createJobForTest(testJobA, s, t)
		createJobForTest(testJobB, s, t)
		createJobForTest(testJobC, s, t)

		data := structs.SearchRequest{Prefix: testJobPrefix, Context: "jobs"}
		req, err := http.NewRequest("POST", "/v1/search", encodeReq(data))
		assert.Nil(err)

		respW := httptest.NewRecorder()

		resp, err := s.Server.SearchRequest(respW, req)
		assert.Nil(err)

		res := resp.(structs.SearchResponse)

		assert.Equal(1, len(res.Matches))

		j := res.Matches["jobs"]

		assert.Equal(2, len(j))
		assert.Contains(j, testJobA)
		assert.Contains(j, testJobB)
		assert.NotContains(j, testJobC)

		assert.Equal(res.Truncations["job"], false)
		assert.NotEqual("0", respW.HeaderMap.Get("X-Nomad-Index"))
	})
}

func TestHTTP_Search_Evaluation(t *testing.T) {
	assert := a.New(t)

	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		state := s.Agent.server.State()
		eval1 := mock.Eval()
		eval2 := mock.Eval()
		err := state.UpsertEvals(9000,
			[]*structs.Evaluation{eval1, eval2})
		assert.Nil(err)

		prefix := eval1.ID[:len(eval1.ID)-2]
		data := structs.SearchRequest{Prefix: prefix, Context: "evals"}
		req, err := http.NewRequest("POST", "/v1/search", encodeReq(data))
		assert.Nil(err)

		respW := httptest.NewRecorder()

		resp, err := s.Server.SearchRequest(respW, req)
		assert.Nil(err)

		res := resp.(structs.SearchResponse)

		assert.Equal(1, len(res.Matches))

		j := res.Matches["evals"]
		assert.Equal(1, len(j))
		assert.Contains(j, eval1.ID)
		assert.NotContains(j, eval2.ID)

		assert.Equal(res.Truncations["evals"], false)
		assert.Equal("9000", respW.HeaderMap.Get("X-Nomad-Index"))
	})
}

func TestHTTP_Search_Allocations(t *testing.T) {
	assert := a.New(t)

	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		state := s.Agent.server.State()
		alloc := mock.Alloc()
		err := state.UpsertAllocs(7000, []*structs.Allocation{alloc})
		assert.Nil(err)

		prefix := alloc.ID[:len(alloc.ID)-2]
		data := structs.SearchRequest{Prefix: prefix, Context: "allocs"}
		req, err := http.NewRequest("POST", "/v1/search", encodeReq(data))
		assert.Nil(err)

		respW := httptest.NewRecorder()

		resp, err := s.Server.SearchRequest(respW, req)
		assert.Nil(err)

		res := resp.(structs.SearchResponse)

		assert.Equal(1, len(res.Matches))

		a := res.Matches["allocs"]
		assert.Equal(1, len(a))
		assert.Contains(a, alloc.ID)

		assert.Equal(res.Truncations["allocs"], false)
		assert.Equal("7000", respW.HeaderMap.Get("X-Nomad-Index"))
	})
}

func TestHTTP_Search_Nodes(t *testing.T) {
	assert := a.New(t)

	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		state := s.Agent.server.State()
		node := mock.Node()
		err := state.UpsertNode(6000, node)
		assert.Nil(err)

		prefix := node.ID[:len(node.ID)-2]
		data := structs.SearchRequest{Prefix: prefix, Context: "nodes"}
		req, err := http.NewRequest("POST", "/v1/search", encodeReq(data))
		assert.Nil(err)

		respW := httptest.NewRecorder()

		resp, err := s.Server.SearchRequest(respW, req)
		assert.Nil(err)

		res := resp.(structs.SearchResponse)

		assert.Equal(1, len(res.Matches))

		n := res.Matches["nodes"]
		assert.Equal(1, len(n))
		assert.Contains(n, node.ID)

		assert.Equal(res.Truncations["nodes"], false)
		assert.Equal("6000", respW.HeaderMap.Get("X-Nomad-Index"))
	})
}

func TestHTTP_Search_NoJob(t *testing.T) {
	assert := a.New(t)

	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		data := structs.SearchRequest{Prefix: "12345", Context: "jobs"}
		req, err := http.NewRequest("POST", "/v1/search", encodeReq(data))
		assert.Nil(err)

		respW := httptest.NewRecorder()

		resp, err := s.Server.SearchRequest(respW, req)
		assert.Nil(err)

		res := resp.(structs.SearchResponse)

		assert.Equal(1, len(res.Matches))
		assert.Equal(0, len(res.Matches["jobs"]))

		assert.Equal("0", respW.HeaderMap.Get("X-Nomad-Index"))
	})
}

func TestHTTP_Search_NoContext(t *testing.T) {
	assert := a.New(t)

	testJobID := "aaaaaaaa-e8f7-fd38-c855-ab94ceb89706"
	testJobPrefix := "aaaaaaaa-e8f7-fd38"
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		createJobForTest(testJobID, s, t)

		state := s.Agent.server.State()
		eval1 := mock.Eval()
		eval1.ID = testJobID
		err := state.UpsertEvals(8000, []*structs.Evaluation{eval1})
		assert.Nil(err)

		data := structs.SearchRequest{Prefix: testJobPrefix}
		req, err := http.NewRequest("POST", "/v1/search", encodeReq(data))
		assert.Nil(err)

		respW := httptest.NewRecorder()

		resp, err := s.Server.SearchRequest(respW, req)
		assert.Nil(err)

		res := resp.(structs.SearchResponse)

		matchedJobs := res.Matches["jobs"]
		matchedEvals := res.Matches["evals"]

		assert.Equal(1, len(matchedJobs))
		assert.Equal(1, len(matchedEvals))

		assert.Equal(matchedJobs[0], testJobID)
		assert.Equal(matchedEvals[0], eval1.ID)

		assert.Equal("8000", respW.HeaderMap.Get("X-Nomad-Index"))
	})
}
