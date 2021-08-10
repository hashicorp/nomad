package spec

import "reflect"

type v1api struct{}

// GetPaths invokes the path configuration logic for each area of the API,
// and aggregates the results into a single set of paths.
func (v *v1api) GetPaths() []*Path {

	paths := v.getJobPaths()
	paths = append(paths, v.getACLPaths()...)
	paths = append(paths, v.getAgentPaths()...)
	paths = append(paths, v.getAllocationPaths()...)
	paths = append(paths, v.getClientPaths()...)
	paths = append(paths, v.getDeploymentsPaths()...)
	paths = append(paths, v.getEnterprisePaths()...)
	paths = append(paths, v.getEvaluationsPaths()...)
	paths = append(paths, v.getMetricsPaths()...)
	paths = append(paths, v.getNamespacePaths()...)
	paths = append(paths, v.getNodePaths()...)
	paths = append(paths, v.getOperatorPaths()...)
	paths = append(paths, v.getPPROFPaths()...)
	paths = append(paths, v.getPluginsPaths()...)
	paths = append(paths, v.getRegionsPaths()...)
	paths = append(paths, v.getScalingPaths()...)
	paths = append(paths, v.getSearchPaths()...)
	paths = append(paths, v.getStatusPaths()...)
	paths = append(paths, v.getSystemPaths()...)
	paths = append(paths, v.getVolumesPaths()...)

	return paths
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
	NamespaceNameParam = Parameter{
		Id:          "NamespaceNameParam",
		SchemaType:  stringSchema,
		Description: "The namespace identifier.",
		Name:        "namespaceName",
		In:          inPath,
		Required:    true,
	}
	QuotaSpecNameParam = Parameter{
		Id:          "QuotaSpecNameParam",
		SchemaType:  stringSchema,
		Description: "The quota spec identifier.",
		Name:        "specName",
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
	IdempotencyTokenParam = Parameter{
		Id:          "IdempotencyTokenParam",
		SchemaType:  stringSchema,
		Description: "Can be used to ensure operations are only run once.",
		Name:        "idempotency_token",
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

var writeOptions = []*Parameter{
	&RegionParam,
	&NamespaceParam,
	&NomadTokenHeader,
	&IdempotencyTokenParam,
}

var (
	BadRequestResponse = Response{
		Name:        "BadRequestResponse",
		Description: "Bad request",
	}
	ForbiddenResponse = Response{
		Name:        "ForbiddenResponse",
		Description: "Forbidden",
	}
	InternalServerErrorResponse = Response{
		Name:        "InternalServerErrorResponse",
		Description: "Internal server error",
	}
	MethodNotAllowedResponse = Response{
		Name:        "MethodNotAllowedResponse",
		Description: "Method not allowed",
	}
	NotFoundResponse = Response{
		Name:        "NotFoundResponse",
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

func newOperation(method string, handler string, tags []string, operationId string, requestBody *RequestBody, params []*Parameter, responses ...*ResponseConfig) *Operation {
	return &Operation{
		Method:      method,
		Handler:     handler,
		Tags:        tags,
		OperationId: operationId,
		RequestBody: requestBody,
		Parameters:  params,
		Responses:   getResponses(responses...),
	}
}

func newRequestBody(schemaType string, model interface{}) *RequestBody {
	return &RequestBody{
		SchemaType: schemaType,
		Model:      reflect.TypeOf(model),
	}
}

func newResponseConfig(statusCode int, schemaType string, model interface{}, headers []*ResponseHeader, name string) *ResponseConfig {
	cfg := &ResponseConfig{
		Code:    statusCode,
		Headers: headers,
		Response: &Response{
			Name: name,
		},
	}

	if model != nil {
		cfg.Content = &Content{
			SchemaType: schemaType,
			Model:      reflect.TypeOf(model),
		}
	}

	return cfg
}

func getResponses(configs ...*ResponseConfig) []*ResponseConfig {
	responses := append(standardResponses, configs...)
	return responses
}
