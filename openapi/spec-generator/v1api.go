package main

import (
	"github.com/hashicorp/nomad/api"
	"net/http"
)

type v1api struct{}

func (v *v1api) getPaths() []*Path {

	paths := v.getJobPaths()
	paths = append(paths, v.getACLPaths()...)
	paths = append(paths, v.getAgentPaths()...)
	paths = append(paths, v.getAllocationPaths()...)
	paths = append(paths, v.getClientPaths()...)
	paths = append(paths, v.getDeploymentsPaths()...)
	paths = append(paths, v.getEnterprisePaths()...)
	paths = append(paths, v.getEvaluationsPaths()...)
	paths = append(paths, v.getMiscPaths()...)
	paths = append(paths, v.getNamespacePaths()...)
	paths = append(paths, v.getNodePaths()...)
	paths = append(paths, v.getOperatorPaths()...)
	paths = append(paths, v.getPPROFPaths()...)
	paths = append(paths, v.getPluginsPaths()...)
	paths = append(paths, v.getVolumesPaths()...)

	return paths
}

func (v *v1api) getJobPaths() []*Path {
	return []*Path{
		{
			Key: "/jobs/{jobName}",
			Operations: []*Operation{
				newOperation(http.MethodGet, []string{"Jobs"}, "getJob",
					nil,
					append(queryOptions, &JobNameParam),
					newResponseConfig(200, objectSchema, api.Job{}, queryMeta, "GetJobResponse"),
				),
			},
		},
		{
			Key: "/jobs",
			Operations: []*Operation{
				newOperation(http.MethodPost, []string{"Jobs"}, "postJob",
					newRequestBody(objectSchema, api.JobRegisterRequest{}),
					nil,
					newResponseConfig(200, objectSchema, api.JobRegisterResponse{}, queryMeta, "PostJobResponse"),
				),
			},
		},
		{
			Key: "/job/{jobName}/plan",
			Operations: []*Operation{
				newOperation(http.MethodPost, []string{"Jobs"}, "postJobPlan",
					newRequestBody(objectSchema, api.JobPlanRequest{}),
					append(queryOptions, &JobNameParam),
					newResponseConfig(200, objectSchema, api.JobPlanResponse{}, queryMeta, "PostJobPlanResponse"),
				),
			},
		},
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

func (v *v1api) getAllocationPaths() []*Path {
	//s.mux.HandleFunc("/v1/allocations", s.wrap(s.AllocsRequest))
	//s.mux.HandleFunc("/v1/allocation/", s.wrap(s.AllocSpecificRequest))

	return nil
}

func (v *v1api) getNodePaths() []*Path {
	//s.mux.HandleFunc("/v1/nodes", s.wrap(s.NodesRequest))
	//s.mux.HandleFunc("/v1/node/", s.wrap(s.NodeSpecificRequest))

	return nil
}

func (v *v1api) getEvaluationsPaths() []*Path {
	//s.mux.HandleFunc("/v1/evaluations", s.wrap(s.EvalsRequest))
	//s.mux.HandleFunc("/v1/evaluation/", s.wrap(s.EvalSpecificRequest))

	return nil
}

func (v *v1api) getDeploymentsPaths() []*Path {
	//s.mux.HandleFunc("/v1/deployments", s.wrap(s.DeploymentsRequest))
	//s.mux.HandleFunc("/v1/deployment/", s.wrap(s.DeploymentSpecificRequest))

	return nil
}

func (v *v1api) getVolumesPaths() []*Path {
	//s.mux.HandleFunc("/v1/volumes", s.wrap(s.CSIVolumesRequest))
	//s.mux.HandleFunc("/v1/volumes/external", s.wrap(s.CSIExternalVolumesRequest))
	//s.mux.HandleFunc("/v1/volumes/snapshot", s.wrap(s.CSISnapshotsRequest))
	//s.mux.HandleFunc("/v1/volume/csi/", s.wrap(s.CSIVolumeSpecificRequest))

	return nil
}

func (v *v1api) getPluginsPaths() []*Path {
	//s.mux.HandleFunc("/v1/plugins", s.wrap(s.CSIPluginsRequest))
	//s.mux.HandleFunc("/v1/plugin/csi/", s.wrap(s.CSIPluginSpecificRequest))

	return nil
}

func (v *v1api) getACLPaths() []*Path {
	//s.mux.HandleFunc("/v1/acl/policies", s.wrap(s.ACLPoliciesRequest))
	//s.mux.HandleFunc("/v1/acl/policy/", s.wrap(s.ACLPolicySpecificRequest))
	//s.mux.HandleFunc("/v1/acl/token/onetime", s.wrap(s.UpsertOneTimeToken))
	//s.mux.HandleFunc("/v1/acl/token/onetime/exchange", s.wrap(s.ExchangeOneTimeToken))
	//s.mux.HandleFunc("/v1/acl/bootstrap", s.wrap(s.ACLTokenBootstrap))
	//s.mux.HandleFunc("/v1/acl/tokens", s.wrap(s.ACLTokensRequest))
	//s.mux.HandleFunc("/v1/acl/token", s.wrap(s.ACLTokenSpecificRequest))
	//s.mux.HandleFunc("/v1/acl/token/", s.wrap(s.ACLTokenSpecificRequest))

	return nil
}

func (v *v1api) getClientPaths() []*Path {
	//s.mux.Handle("/v1/client/fs/", wrapCORS(s.wrap(s.FsRequest)))
	//s.mux.HandleFunc("/v1/client/gc", s.wrap(s.ClientGCRequest))
	//s.mux.Handle("/v1/client/stats", wrapCORS(s.wrap(s.ClientStatsRequest)))
	//s.mux.Handle("/v1/client/allocation/", wrapCORS(s.wrap(s.ClientAllocRequest)))

	return nil
}

func (v *v1api) getAgentPaths() []*Path {
	//s.mux.HandleFunc("/v1/agent/self", s.wrap(s.AgentSelfRequest))
	//s.mux.HandleFunc("/v1/agent/join", s.wrap(s.AgentJoinRequest))
	//s.mux.HandleFunc("/v1/agent/members", s.wrap(s.AgentMembersRequest))
	//s.mux.HandleFunc("/v1/agent/force-leave", s.wrap(s.AgentForceLeaveRequest))
	//s.mux.HandleFunc("/v1/agent/servers", s.wrap(s.AgentServersRequest))
	//s.mux.HandleFunc("/v1/agent/keyring/", s.wrap(s.KeyringOperationRequest))
	//s.mux.HandleFunc("/v1/agent/health", s.wrap(s.HealthRequest))
	//s.mux.HandleFunc("/v1/agent/host", s.wrap(s.AgentHostRequest))
	//s.mux.HandleFunc("/v1/agent/monitor", s.wrap(s.AgentMonitor))
	//s.mux.HandleFunc("/v1/agent/pprof/", s.wrapNonJSON(s.AgentPprofRequest))

	return nil
}

func (v *v1api) getMiscPaths() []*Path {

	//s.mux.HandleFunc("/v1/metrics", s.wrap(s.MetricsRequest))
	//s.mux.HandleFunc("/v1/regions", s.wrap(s.RegionListRequest))
	//
	//s.mux.HandleFunc("/v1/scaling/policies", s.wrap(s.ScalingPoliciesRequest))
	//s.mux.HandleFunc("/v1/scaling/policy/", s.wrap(s.ScalingPolicySpecificRequest))
	//
	//s.mux.HandleFunc("/v1/status/leader", s.wrap(s.StatusLeaderRequest))
	//s.mux.HandleFunc("/v1/status/peers", s.wrap(s.StatusPeersRequest))
	//
	//s.mux.HandleFunc("/v1/search/fuzzy", s.wrap(s.FuzzySearchRequest))
	//s.mux.HandleFunc("/v1/search", s.wrap(s.SearchRequest))

	//s.mux.HandleFunc("/v1/system/gc", s.wrap(s.GarbageCollectRequest))
	//s.mux.HandleFunc("/v1/system/reconcile/summaries", s.wrap(s.ReconcileJobSummaries))

	//s.mux.HandleFunc("/v1/event/stream", s.wrap(s.EventStream))

	return nil
}

func (v *v1api) getOperatorPaths() []*Path {
	//s.mux.HandleFunc("/v1/operator/license", s.wrap(s.LicenseRequest))
	//s.mux.HandleFunc("/v1/operator/raft/", s.wrap(s.OperatorRequest))
	//s.mux.HandleFunc("/v1/operator/autopilot/configuration", s.wrap(s.OperatorAutopilotConfiguration))
	//s.mux.HandleFunc("/v1/operator/autopilot/health", s.wrap(s.OperatorServerHealth))
	//s.mux.HandleFunc("/v1/operator/snapshot", s.wrap(s.SnapshotRequest))
	//s.mux.HandleFunc("/v1/operator/scheduler/configuration", s.wrap(s.OperatorSchedulerConfiguration))

	return nil
}

func (v *v1api) getNamespacePaths() []*Path {

	//s.mux.HandleFunc("/v1/namespaces", s.wrap(s.NamespacesRequest))
	//s.mux.HandleFunc("/v1/namespace", s.wrap(s.NamespaceCreateRequest))
	//s.mux.HandleFunc("/v1/namespace/", s.wrap(s.NamespaceSpecificRequest))

	return nil
}

func (v *v1api) getPPROFPaths() []*Path {
	//s.mux.HandleFunc("/debug/pprof/", pprof.Index)
	//s.mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	//s.mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	//s.mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	//s.mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	return nil
}

func (v *v1api) getEnterprisePaths() []*Path {
	//s.mux.HandleFunc("/v1/sentinel/policies", s.wrap(s.entOnly))
	//s.mux.HandleFunc("/v1/sentinel/policy/", s.wrap(s.entOnly))
	//
	//s.mux.HandleFunc("/v1/quotas", s.wrap(s.entOnly))
	//s.mux.HandleFunc("/v1/quota-usages", s.wrap(s.entOnly))
	//s.mux.HandleFunc("/v1/quota/", s.wrap(s.entOnly))
	//s.mux.HandleFunc("/v1/quota", s.wrap(s.entOnly))
	//
	//s.mux.HandleFunc("/v1/recommendation", s.wrap(s.entOnly))
	//s.mux.HandleFunc("/v1/recommendations", s.wrap(s.entOnly))
	//s.mux.HandleFunc("/v1/recommendations/apply", s.wrap(s.entOnly))
	//s.mux.HandleFunc("/v1/recommendation/", s.wrap(s.entOnly))

	return nil
}

var (
	AllParam = Parameter{
		Id:          "AllParam",
		SchemaType:  intSchema,
		Description: "Flag indicating whether to constrain by job creation index or not.",
		Name:        "all",
		In:          inQuery,
	}
	IndexHeader = Parameter{
		Id:          "IndexHeader",
		SchemaType:  intSchema,
		Description: "If set, wait until query exceeds given index. Must be provided with WaitParam.",
		Name:        "index",
		In:          inHeader,
	}
	JobNameParam = Parameter{
		Id:          "JobNameParam",
		SchemaType:  stringSchema,
		Description: "The job identifier.",
		Name:        "jobName",
		In:          inPath,
		Required:    true,
	}
	NamespaceParam = Parameter{
		Id:          "NamespaceParam",
		SchemaType:  stringSchema,
		Description: "Filters results based on the specified namespace.",
		Name:        "namespace",
		In:          inQuery,
	}
	NextTokenParam = Parameter{
		Id:          "NextTokenParam",
		SchemaType:  stringSchema,
		Description: "Indicates where to start paging for queries that support pagination.",
		Name:        "next_token",
		In:          inQuery,
	}
	PerPageParam = Parameter{
		Id:          "PerPageParam",
		SchemaType:  intSchema,
		Description: "Maximum number of results to return.",
		Name:        "per_page",
		In:          inQuery,
	}
	PrefixParam = Parameter{
		Id:          "PrefixParam",
		SchemaType:  stringSchema,
		Description: "Constrains results to jobs that start with the defined prefix",
		Name:        "prefix",
		In:          inQuery,
	}
	RegionParam = Parameter{
		Id:          "RegionParam",
		SchemaType:  stringSchema,
		Description: "Filters results based on the specified region.",
		Name:        "region",
		In:          inQuery,
	}
	StaleParam = Parameter{
		Id:          "StaleParam",
		SchemaType:  stringSchema,
		Description: "If present, results will include stale reads.",
		Name:        "stale",
		In:          inQuery,
	}
	WaitParam = Parameter{
		Id:          "WaitParam",
		SchemaType:  intSchema,
		Description: "Provided with IndexParam to wait for change.",
		Name:        "wait",
		In:          inQuery,
	}
	NomadTokenHeader = Parameter{
		Id:          "NextTokenHeader",
		SchemaType:  stringSchema,
		Description: "A Nomad ACL token.",
		Name:        "X-Nomad-Token",
		In:          inHeader,
	}
	KnownLeaderHeader = Parameter{
		Id:          "KnownLeaderHeader",
		Name:        "X-Nomad-Known-Leader",
		SchemaType:  boolSchema,
		Description: "",
		In:          inHeader,
	}
	LastContactHeader = Parameter{
		Id:          "LastContactHeader",
		Name:        "X-Nomad-Last-Contact",
		SchemaType:  intSchema,
		Description: "",
		In:          inHeader,
	}
)

var (
	XNomadIndexHeader = ResponseHeader{
		Name:        "X-Nomad-Index",
		SchemaType:  intSchema,
		Description: "A unique identifier representing the current state of the requested resource. On a new Nomad cluster the value of this index starts at 1.",
	}
	XNomadKnownLeaderHeader = ResponseHeader{
		Name:        "X-Nomad-KnownLeader",
		SchemaType:  boolSchema,
		Description: "Boolean indicating if there is a known cluster leader.",
	}
	XNomadLastContactHeader = ResponseHeader{
		Name:        "X-Nomad-LastContact",
		SchemaType:  intSchema,
		Description: "The time in milliseconds that a server was last contacted by the leader node.",
	}
)

var queryMeta = []*ResponseHeader{
	&XNomadIndexHeader,
	&XNomadKnownLeaderHeader,
	&XNomadLastContactHeader,
}

var writeMeta = []*ResponseHeader{
	&XNomadIndexHeader,
}

var queryOptions = []*Parameter{
	&RegionParam,
	&NamespaceParam,
	&IndexHeader,
	&WaitParam,
	&StaleParam,
	&PrefixParam,
	&NomadTokenHeader,
	&PerPageParam,
	&NextTokenParam,
}

var (
	BadRequestRespnse = Response{
		Id:          "BadRequestResponse",
		Description: "Bad request",
	}
	ForbiddenResponse = Response{
		Id:          "ForbiddenResponse",
		Description: "Forbidden",
	}
	InternalServerErrorResponse = Response{
		Id:          "InternalServerErrorResponse",
		Description: "Internal server error",
	}
	MethodNotAllowedResponse = Response{
		Id:          "MethodNotAllowedResponse",
		Description: "Method not allowed",
	}
	NotFoundResponse = Response{
		Id:          "NotFoundResponse",
		Description: "Not found",
	}
)

var standardResponses = []*ResponseConfig{
	{
		Code:     403,
		Response: &ForbiddenResponse,
	},
	{
		Code:     500,
		Response: &InternalServerErrorResponse,
	},
	{
		Code:     405,
		Response: &MethodNotAllowedResponse,
	},
}
