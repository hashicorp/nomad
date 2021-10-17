package agent

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

func header(recorder *httptest.ResponseRecorder, name string) string {
	return recorder.Result().Header.Get(name)
}

func createJobForTest(jobID string, s *TestAgent, t *testing.T) {
	job := mock.Job()
	job.ID = jobID
	job.TaskGroups[0].Count = 1
	state := s.Agent.server.State()
	err := state.UpsertJob(structs.MsgTypeTestSetup, 1000, job)
	require.NoError(t, err)
}

func TestHTTP_PrefixSearchWithIllegalMethod(t *testing.T) {
	t.Parallel()

	httpTest(t, nil, func(s *TestAgent) {
		req, err := http.NewRequest("DELETE", "/v1/search", nil)
		require.NoError(t, err)
		respW := httptest.NewRecorder()

		_, err = s.Server.SearchRequest(respW, req)
		require.EqualError(t, err, "Invalid method")
	})
}

func TestHTTP_FuzzySearchWithIllegalMethod(t *testing.T) {
	t.Parallel()

	httpTest(t, nil, func(s *TestAgent) {
		req, err := http.NewRequest("DELETE", "/v1/search/fuzzy", nil)
		require.NoError(t, err)
		respW := httptest.NewRecorder()

		_, err = s.Server.SearchRequest(respW, req)
		require.EqualError(t, err, "Invalid method")
	})
}

func createCmdJobForTest(name, cmd string, s *TestAgent, t *testing.T) *structs.Job {
	job := mock.Job()
	job.Name = name
	job.TaskGroups[0].Tasks[0].Config["command"] = cmd
	job.TaskGroups[0].Count = 1
	state := s.Agent.server.State()
	err := state.UpsertJob(structs.MsgTypeTestSetup, 1000, job)
	require.NoError(t, err)
	return job
}

func TestHTTP_PrefixSearch_POST(t *testing.T) {
	t.Parallel()

	testJob := "aaaaaaaa-e8f7-fd38-c855-ab94ceb89706"
	testJobPrefix := "aaaaaaaa-e8f7-fd38"

	httpTest(t, nil, func(s *TestAgent) {
		createJobForTest(testJob, s, t)

		data := structs.SearchRequest{Prefix: testJobPrefix, Context: structs.Jobs}
		req, err := http.NewRequest("POST", "/v1/search", encodeReq(data))
		require.NoError(t, err)

		respW := httptest.NewRecorder()

		resp, err := s.Server.SearchRequest(respW, req)
		require.NoError(t, err)

		res := resp.(structs.SearchResponse)
		require.Len(t, res.Matches, 1)

		j := res.Matches[structs.Jobs]
		require.Len(t, j, 1)
		require.Equal(t, testJob, j[0])

		require.False(t, res.Truncations[structs.Jobs])
		require.NotEqual(t, "0", header(respW, "X-Nomad-Index"))
	})
}

func TestHTTP_FuzzySearch_POST(t *testing.T) {
	t.Parallel()

	testJobID := uuid.Generate()

	httpTest(t, nil, func(s *TestAgent) {
		createJobForTest(testJobID, s, t)
		data := structs.FuzzySearchRequest{Text: "fau", Context: structs.Namespaces}
		req, err := http.NewRequest("POST", "/v1/search/fuzzy", encodeReq(data))
		require.NoError(t, err)

		respW := httptest.NewRecorder()

		resp, err := s.Server.FuzzySearchRequest(respW, req)
		require.NoError(t, err)

		res := resp.(structs.FuzzySearchResponse)
		require.Len(t, res.Matches, 1) // searched one context: namespaces

		ns := res.Matches[structs.Namespaces]
		require.Len(t, ns, 1)

		require.Equal(t, "default", ns[0].ID)
		require.Nil(t, ns[0].Scope) // only job types have scope

		require.False(t, res.Truncations[structs.Jobs])
		require.NotEqual(t, "0", header(respW, "X-Nomad-Index"))
	})
}

func TestHTTP_PrefixSearch_PUT(t *testing.T) {
	t.Parallel()

	testJob := "aaaaaaaa-e8f7-fd38-c855-ab94ceb89706"
	testJobPrefix := "aaaaaaaa-e8f7-fd38"

	httpTest(t, nil, func(s *TestAgent) {
		createJobForTest(testJob, s, t)

		data := structs.SearchRequest{Prefix: testJobPrefix, Context: structs.Jobs}
		req, err := http.NewRequest("PUT", "/v1/search", encodeReq(data))
		require.NoError(t, err)

		respW := httptest.NewRecorder()

		resp, err := s.Server.SearchRequest(respW, req)
		require.NoError(t, err)

		res := resp.(structs.SearchResponse)
		require.Len(t, res.Matches, 1)

		j := res.Matches[structs.Jobs]
		require.Len(t, j, 1)
		require.Equal(t, testJob, j[0])

		require.False(t, res.Truncations[structs.Jobs])
		require.NotEqual(t, "0", header(respW, "X-Nomad-Index"))
	})
}

func TestHTTP_FuzzySearch_PUT(t *testing.T) {
	t.Parallel()

	testJobID := uuid.Generate()

	httpTest(t, nil, func(s *TestAgent) {
		createJobForTest(testJobID, s, t)
		data := structs.FuzzySearchRequest{Text: "fau", Context: structs.Namespaces}
		req, err := http.NewRequest("PUT", "/v1/search/fuzzy", encodeReq(data))
		require.NoError(t, err)

		respW := httptest.NewRecorder()

		resp, err := s.Server.FuzzySearchRequest(respW, req)
		require.NoError(t, err)

		res := resp.(structs.FuzzySearchResponse)
		require.Len(t, res.Matches, 1) // searched one context: namespaces

		ns := res.Matches[structs.Namespaces]
		require.Len(t, ns, 1)

		require.Equal(t, "default", ns[0].ID)
		require.Nil(t, ns[0].Scope) // only job types have scope

		require.False(t, res.Truncations[structs.Namespaces])
		require.NotEqual(t, "0", header(respW, "X-Nomad-Index"))
	})
}

func TestHTTP_PrefixSearch_MultipleJobs(t *testing.T) {
	t.Parallel()

	testJobA := "aaaaaaaa-e8f7-fd38-c855-ab94ceb89706"
	testJobB := "aaaaaaaa-e8f7-fd38-c855-ab94ceb89707"
	testJobC := "bbbbbbbb-e8f7-fd38-c855-ab94ceb89707"
	testJobPrefix := "aaaaaaaa-e8f7-fd38"

	httpTest(t, nil, func(s *TestAgent) {
		createJobForTest(testJobA, s, t)
		createJobForTest(testJobB, s, t)
		createJobForTest(testJobC, s, t)

		data := structs.SearchRequest{Prefix: testJobPrefix, Context: structs.Jobs}
		req, err := http.NewRequest("POST", "/v1/search", encodeReq(data))
		require.NoError(t, err)

		respW := httptest.NewRecorder()

		resp, err := s.Server.SearchRequest(respW, req)
		require.NoError(t, err)

		res := resp.(structs.SearchResponse)
		require.Len(t, res.Matches, 1)

		j := res.Matches[structs.Jobs]
		require.Len(t, j, 2)
		require.Contains(t, j, testJobA)
		require.Contains(t, j, testJobB)
		require.NotContains(t, j, testJobC)

		require.False(t, res.Truncations[structs.Jobs])
		require.NotEqual(t, "0", header(respW, "X-Nomad-Index"))
	})
}

func TestHTTP_FuzzySearch_MultipleJobs(t *testing.T) {
	t.Parallel()

	httpTest(t, nil, func(s *TestAgent) {
		job1ID := createCmdJobForTest("job1", "/bin/yes", s, t).ID
		job2ID := createCmdJobForTest("job2", "/bin/no", s, t).ID
		_ = createCmdJobForTest("job3", "/opt/java", s, t).ID // no match
		job4ID := createCmdJobForTest("job4", "/sbin/ping", s, t).ID

		data := structs.FuzzySearchRequest{Text: "bin", Context: structs.Jobs}
		req, err := http.NewRequest("POST", "/v1/search/fuzzy", encodeReq(data))
		require.NoError(t, err)

		respW := httptest.NewRecorder()

		resp, err := s.Server.FuzzySearchRequest(respW, req)
		require.NoError(t, err)

		// in example job, only the commands match the "bin" query

		res := resp.(structs.FuzzySearchResponse)
		require.Len(t, res.Matches, 1)

		commands := res.Matches[structs.Commands]
		require.Len(t, commands, 3)

		exp := []structs.FuzzyMatch{{
			ID:    "/bin/no",
			Scope: []string{"default", job2ID, "web", "web"},
		}, {
			ID:    "/bin/yes",
			Scope: []string{"default", job1ID, "web", "web"},
		}, {
			ID:    "/sbin/ping",
			Scope: []string{"default", job4ID, "web", "web"},
		}}
		require.Equal(t, exp, commands)

		require.False(t, res.Truncations[structs.Jobs])
		require.NotEqual(t, "0", header(respW, "X-Nomad-Index"))
	})
}

func TestHTTP_PrefixSearch_Evaluation(t *testing.T) {
	t.Parallel()

	httpTest(t, nil, func(s *TestAgent) {
		state := s.Agent.server.State()
		eval1 := mock.Eval()
		eval2 := mock.Eval()
		err := state.UpsertEvals(structs.MsgTypeTestSetup, 9000, []*structs.Evaluation{eval1, eval2})
		require.NoError(t, err)

		prefix := eval1.ID[:len(eval1.ID)-2]
		data := structs.SearchRequest{Prefix: prefix, Context: structs.Evals}
		req, err := http.NewRequest("POST", "/v1/search", encodeReq(data))
		require.NoError(t, err)

		respW := httptest.NewRecorder()

		resp, err := s.Server.SearchRequest(respW, req)
		require.NoError(t, err)

		res := resp.(structs.SearchResponse)
		require.Len(t, res.Matches, 1)

		j := res.Matches[structs.Evals]
		require.Len(t, j, 1)
		require.Contains(t, j, eval1.ID)
		require.NotContains(t, j, eval2.ID)
		require.False(t, res.Truncations[structs.Evals])
		require.Equal(t, "9000", header(respW, "X-Nomad-Index"))
	})
}

func TestHTTP_FuzzySearch_Evaluation(t *testing.T) {
	t.Parallel()

	httpTest(t, nil, func(s *TestAgent) {
		state := s.Agent.server.State()
		eval1 := mock.Eval()
		eval2 := mock.Eval()
		err := state.UpsertEvals(structs.MsgTypeTestSetup, 9000, []*structs.Evaluation{eval1, eval2})
		require.NoError(t, err)

		// fuzzy search does prefix search for evaluations
		prefix := eval1.ID[:len(eval1.ID)-2]
		data := structs.FuzzySearchRequest{Text: prefix, Context: structs.Evals}
		req, err := http.NewRequest("POST", "/v1/search/fuzzy", encodeReq(data))
		require.NoError(t, err)

		respW := httptest.NewRecorder()

		resp, err := s.Server.FuzzySearchRequest(respW, req)
		require.NoError(t, err)

		res := resp.(structs.FuzzySearchResponse)
		require.Len(t, res.Matches, 1)

		matches := res.Matches[structs.Evals]
		require.Len(t, matches, 1)

		require.Equal(t, structs.FuzzyMatch{
			ID: eval1.ID,
		}, matches[0])
		require.False(t, res.Truncations[structs.Evals])
		require.Equal(t, "9000", header(respW, "X-Nomad-Index"))
	})
}

func mockAlloc() *structs.Allocation {
	a := mock.Alloc()
	a.Name = fmt.Sprintf("%s.%s[%d]", a.Job.Name, "web", 0)
	return a
}

func TestHTTP_PrefixSearch_Allocations(t *testing.T) {
	t.Parallel()

	httpTest(t, nil, func(s *TestAgent) {
		state := s.Agent.server.State()
		alloc := mockAlloc()
		err := state.UpsertAllocs(structs.MsgTypeTestSetup, 7000, []*structs.Allocation{alloc})
		require.NoError(t, err)

		prefix := alloc.ID[:len(alloc.ID)-2]
		data := structs.SearchRequest{Prefix: prefix, Context: structs.Allocs}
		req, err := http.NewRequest("POST", "/v1/search", encodeReq(data))
		require.NoError(t, err)

		respW := httptest.NewRecorder()

		resp, err := s.Server.SearchRequest(respW, req)
		require.NoError(t, err)

		res := resp.(structs.SearchResponse)
		require.Len(t, res.Matches, 1)

		a := res.Matches[structs.Allocs]
		require.Len(t, a, 1)
		require.Contains(t, a, alloc.ID)

		require.False(t, res.Truncations[structs.Allocs])
		require.Equal(t, "7000", header(respW, "X-Nomad-Index"))
	})
}

func TestHTTP_FuzzySearch_Allocations(t *testing.T) {
	t.Parallel()

	httpTest(t, nil, func(s *TestAgent) {
		state := s.Agent.server.State()
		alloc := mockAlloc()
		err := state.UpsertAllocs(structs.MsgTypeTestSetup, 7000, []*structs.Allocation{alloc})
		require.NoError(t, err)

		data := structs.FuzzySearchRequest{Text: "-job", Context: structs.Allocs}
		req, err := http.NewRequest("POST", "/v1/search/fuzzy", encodeReq(data))
		require.NoError(t, err)

		respW := httptest.NewRecorder()

		resp, err := s.Server.FuzzySearchRequest(respW, req)
		require.NoError(t, err)

		res := resp.(structs.FuzzySearchResponse)
		require.Len(t, res.Matches, 1)

		a := res.Matches[structs.Allocs]
		require.Len(t, a, 1)
		require.Equal(t, "my-job.web[0]", a[0].ID)

		require.False(t, res.Truncations[structs.Allocs])
		require.Equal(t, "7000", header(respW, "X-Nomad-Index"))
	})
}

func TestHTTP_PrefixSearch_Nodes(t *testing.T) {
	t.Parallel()

	httpTest(t, nil, func(s *TestAgent) {
		state := s.Agent.server.State()
		node := mock.Node()
		err := state.UpsertNode(structs.MsgTypeTestSetup, 6000, node)
		require.NoError(t, err)

		prefix := node.ID[:len(node.ID)-2]
		data := structs.SearchRequest{Prefix: prefix, Context: structs.Nodes}
		req, err := http.NewRequest("POST", "/v1/search", encodeReq(data))
		require.NoError(t, err)

		respW := httptest.NewRecorder()

		resp, err := s.Server.SearchRequest(respW, req)
		require.NoError(t, err)

		res := resp.(structs.SearchResponse)
		require.Len(t, res.Matches, 1)

		n := res.Matches[structs.Nodes]
		require.Len(t, n, 1)
		require.Contains(t, n, node.ID)

		require.False(t, res.Truncations[structs.Nodes])
		require.Equal(t, "6000", header(respW, "X-Nomad-Index"))
	})
}

func TestHTTP_FuzzySearch_Nodes(t *testing.T) {
	t.Parallel()

	httpTest(t, nil, func(s *TestAgent) {
		state := s.Agent.server.State()
		node := mock.Node() // foobar
		err := state.UpsertNode(structs.MsgTypeTestSetup, 6000, node)
		require.NoError(t, err)

		data := structs.FuzzySearchRequest{Text: "oo", Context: structs.Nodes}
		req, err := http.NewRequest("POST", "/v1/search/fuzzy", encodeReq(data))
		require.NoError(t, err)

		respW := httptest.NewRecorder()

		resp, err := s.Server.FuzzySearchRequest(respW, req)
		require.NoError(t, err)

		res := resp.(structs.FuzzySearchResponse)
		require.Len(t, res.Matches, 1)

		n := res.Matches[structs.Nodes]
		require.Len(t, n, 1)
		require.Equal(t, "foobar", n[0].ID)

		require.False(t, res.Truncations[structs.Nodes])
		require.Equal(t, "6000", header(respW, "X-Nomad-Index"))
	})
}

func TestHTTP_PrefixSearch_Deployments(t *testing.T) {
	t.Parallel()

	httpTest(t, nil, func(s *TestAgent) {
		state := s.Agent.server.State()
		deployment := mock.Deployment()
		require.NoError(t, state.UpsertDeployment(999, deployment), "UpsertDeployment")

		prefix := deployment.ID[:len(deployment.ID)-2]
		data := structs.SearchRequest{Prefix: prefix, Context: structs.Deployments}
		req, err := http.NewRequest("POST", "/v1/search", encodeReq(data))
		require.NoError(t, err)

		respW := httptest.NewRecorder()

		resp, err := s.Server.SearchRequest(respW, req)
		require.NoError(t, err)

		res := resp.(structs.SearchResponse)
		require.Len(t, res.Matches, 1)

		n := res.Matches[structs.Deployments]
		require.Len(t, n, 1)
		require.Contains(t, n, deployment.ID)
		require.Equal(t, "999", header(respW, "X-Nomad-Index"))
	})
}

func TestHTTP_FuzzySearch_Deployments(t *testing.T) {
	t.Parallel()

	httpTest(t, nil, func(s *TestAgent) {
		state := s.Agent.server.State()
		deployment := mock.Deployment()
		require.NoError(t, state.UpsertDeployment(999, deployment), "UpsertDeployment")

		// fuzzy search of deployments are prefix searches
		prefix := deployment.ID[:len(deployment.ID)-2]
		data := structs.FuzzySearchRequest{Text: prefix, Context: structs.Deployments}
		req, err := http.NewRequest("POST", "/v1/search/fuzzy", encodeReq(data))
		require.NoError(t, err)

		respW := httptest.NewRecorder()

		resp, err := s.Server.FuzzySearchRequest(respW, req)
		require.NoError(t, err)

		res := resp.(structs.FuzzySearchResponse)
		require.Len(t, res.Matches, 1)

		n := res.Matches[structs.Deployments]
		require.Len(t, n, 1)
		require.Equal(t, deployment.ID, n[0].ID)
		require.Equal(t, "999", header(respW, "X-Nomad-Index"))
	})
}

func TestHTTP_PrefixSearch_NoJob(t *testing.T) {
	t.Parallel()

	httpTest(t, nil, func(s *TestAgent) {
		data := structs.SearchRequest{Prefix: "12345", Context: structs.Jobs}
		req, err := http.NewRequest("POST", "/v1/search", encodeReq(data))
		require.NoError(t, err)

		respW := httptest.NewRecorder()

		resp, err := s.Server.SearchRequest(respW, req)
		require.NoError(t, err)

		res := resp.(structs.SearchResponse)
		require.Len(t, res.Matches, 1)
		require.Len(t, res.Matches[structs.Jobs], 0)
		require.Equal(t, "0", header(respW, "X-Nomad-Index"))
	})
}

func TestHTTP_FuzzySearch_NoJob(t *testing.T) {
	t.Parallel()

	httpTest(t, nil, func(s *TestAgent) {
		data := structs.FuzzySearchRequest{Text: "12345", Context: structs.Jobs}
		req, err := http.NewRequest("POST", "/v1/search/fuzzy", encodeReq(data))
		require.NoError(t, err)

		respW := httptest.NewRecorder()

		resp, err := s.Server.FuzzySearchRequest(respW, req)
		require.NoError(t, err)

		res := resp.(structs.FuzzySearchResponse)
		require.Len(t, res.Matches, 0)
		require.Equal(t, "0", header(respW, "X-Nomad-Index"))
	})
}

func TestHTTP_PrefixSearch_AllContext(t *testing.T) {
	t.Parallel()

	testJobID := "aaaaaaaa-e8f7-fd38-c855-ab94ceb89706"
	testJobPrefix := "aaaaaaaa-e8f7-fd38"

	httpTest(t, nil, func(s *TestAgent) {
		createJobForTest(testJobID, s, t)

		state := s.Agent.server.State()
		eval1 := mock.Eval()
		eval1.ID = testJobID
		err := state.UpsertEvals(structs.MsgTypeTestSetup, 8000, []*structs.Evaluation{eval1})
		require.NoError(t, err)

		data := structs.SearchRequest{Prefix: testJobPrefix, Context: structs.All}
		req, err := http.NewRequest("POST", "/v1/search", encodeReq(data))
		require.NoError(t, err)

		respW := httptest.NewRecorder()

		resp, err := s.Server.SearchRequest(respW, req)
		require.NoError(t, err)

		res := resp.(structs.SearchResponse)
		matchedJobs := res.Matches[structs.Jobs]
		matchedEvals := res.Matches[structs.Evals]
		require.Len(t, matchedJobs, 1)
		require.Len(t, matchedEvals, 1)
		require.Equal(t, testJobID, matchedJobs[0])
		require.Equal(t, eval1.ID, matchedEvals[0])
		require.Equal(t, "8000", header(respW, "X-Nomad-Index"))
	})
}

func TestHTTP_FuzzySearch_AllContext(t *testing.T) {
	t.Parallel()

	httpTest(t, nil, func(s *TestAgent) {
		jobID := createCmdJobForTest("job1", "/bin/aardvark", s, t).ID

		state := s.Agent.server.State()
		eval1 := mock.Eval()
		eval1.ID = "aaaa6573-04cb-61b4-04cb-865aaaf5d400"
		err := state.UpsertEvals(structs.MsgTypeTestSetup, 8000, []*structs.Evaluation{eval1})
		require.NoError(t, err)

		data := structs.FuzzySearchRequest{Text: "aa", Context: structs.All}
		req, err := http.NewRequest("POST", "/v1/search/fuzzy", encodeReq(data))
		require.NoError(t, err)

		respW := httptest.NewRecorder()

		resp, err := s.Server.FuzzySearchRequest(respW, req)
		require.NoError(t, err)

		res := resp.(structs.FuzzySearchResponse)
		matchedCommands := res.Matches[structs.Commands]
		matchedEvals := res.Matches[structs.Evals]
		require.Len(t, matchedCommands, 1)
		require.Len(t, matchedEvals, 1)
		require.Equal(t, eval1.ID, matchedEvals[0].ID)
		require.Equal(t, "/bin/aardvark", matchedCommands[0].ID)
		require.Equal(t, []string{
			"default", jobID, "web", "web",
		}, matchedCommands[0].Scope)
		require.Equal(t, "8000", header(respW, "X-Nomad-Index"))
	})
}
