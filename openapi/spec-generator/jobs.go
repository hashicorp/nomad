package main

import (
	"net/http"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/nomad/structs"
)

func (v *v1api) getJobPaths() []*Path {
	tags := []string{"Jobs"}

	return []*Path{
		{
			Template: "/jobs",
			Operations: []*Operation{
				newOperation(http.MethodGet, "jobListRequest", tags, "GetJobs",
					newRequestBody(objectSchema, structs.JobListRequest{}),
					queryOptions,
					newResponseConfig(200, arraySchema, api.JobListStub{}, queryMeta, "GetJobsResponse"),
				),
				newOperation(http.MethodPost, "jobUpdate", tags, "PostJob",
					newRequestBody(objectSchema, api.JobRegisterRequest{}),
					nil,
					newResponseConfig(200, objectSchema, api.JobRegisterResponse{}, queryMeta, "PostJobResponse"),
				),
			},
		},
		{
			Template: "/job/{jobName}/plan",
			Operations: []*Operation{
				newOperation(http.MethodPost, "jobPlan", tags, "PostJobPlan",
					newRequestBody(objectSchema, api.JobPlanRequest{}),
					append(queryOptions, &JobNameParam),
					newResponseConfig(200, objectSchema, api.JobPlanResponse{}, queryMeta, "PostJobPlanResponse"),
				),
			},
		},
		// "/v1/jobs/parse",
		// "/job/{jobName}/evaluate"):
		// "/job/{jobName}/evaluate")
		//	"/job/{jobName}/allocations")
		//	"/job/{jobName}/evaluations")
		//	"/job/{jobName}/periodic/force")
		//	"/job/{jobName}/plan")
		//	"/job/{jobName}/summary")
		//	"/job/{jobName}/dispatch")
		//	"/job/{jobName}/versions")
		//  "/job/{jobName}/revert")
		//	"/job/{jobName}/deployments")
		//	"/job/{jobName}/deployment")
		//	"/job/{jobName}/stable")
		//	"/job/{jobName}/scale")
		//s.mux.HandleFunc("/v1/jobs/parse", s.wrap(s.JobsParseRequest))
		//s.mux.HandleFunc("/v1/job/", s.wrap(s.JobSpecificRequest))
		//s.mux.HandleFunc("/v1/validate/job", s.wrap(s.ValidateJobRequest))
	}
}
