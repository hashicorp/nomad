package agent

import (
	"fmt"
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
	args := structs.JobRegisterRequest{
		Job:          job,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var resp structs.JobRegisterResponse
	if err := s.Agent.RPC("Job.Register", &args, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestHTTP_ResourcesWithSingleJob(t *testing.T) {
	testJob := "aaaaaaaa-e8f7-fd38-c855-ab94ceb89706"
	t.Parallel()
	httpTest(t, nil, func(s *TestAgent) {
		createJobForTest(testJob, s, t)

		endpoint := fmt.Sprintf("/v1/resources?context=job&prefix=%s", testJob)
		req, err := http.NewRequest("GET", endpoint, nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respW := httptest.NewRecorder()

		resp, err := s.Server.ResourcesRequest(respW, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		res := resp.(*structs.ResourcesListStub)
		if len(res.Matches) != 1 {
			t.Fatalf("No expected key values in resources list")
		}

		j := res.Matches["jobs"]
		if j == nil || len(j) != 1 {
			t.Fatalf("The number of jobs that were returned does not equal the number of jobs we expected (1)", j)
		}

		// TODO verify that the job we are getting is the same that we created
		//	assert.Equal(t, j[0], testJob)
	})
}

//
//func TestHTTP_ResourcesWithNoJob(t *testing.T) {
//}
