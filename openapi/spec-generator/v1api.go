package main

import "reflect"

type v1api struct{}

// GetPaths invokes the path configuration logic for each area of the API,
// and aggregates the results into a single set of paths.
func (v *v1api) GetPaths() []*apiPath {

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
	allParam = parameter{
		Id:          "AllParam",
		SchemaType:  intSchema,
		Description: "Flag indicating whether to constrain by job creation index or not.",
		Name:        "all",
		In:          inQuery,
	}
	indexHeader = parameter{
		Id:          "IndexHeader",
		SchemaType:  intSchema,
		Description: "If set, wait until query exceeds given index. Must be provided with WaitParam.",
		Name:        "index",
		In:          inHeader,
	}
	jobNameParam = parameter{
		Id:          "JobNameParam",
		SchemaType:  stringSchema,
		Description: "The job identifier.",
		Name:        "jobName",
		In:          inPath,
		Required:    true,
	}
	namespaceNameParam = parameter{
		Id:          "NamespaceNameParam",
		SchemaType:  stringSchema,
		Description: "The namespace identifier.",
		Name:        "namespaceName",
		In:          inPath,
		Required:    true,
	}
	metricsSummaryFormatParam = parameter{
		Id:          "MetricsFormatParam",
		SchemaType:  stringSchema,
		Description: "The format the user requested for the metrics summary (e.g. prometheus)",
		Name:        "format",
		In:          inQuery,
		Required:    false,
	}
	quotaSpecNameParam = parameter{
		Id:          "QuotaSpecNameParam",
		SchemaType:  stringSchema,
		Description: "The quota spec identifier.",
		Name:        "specName",
		In:          inPath,
		Required:    true,
	}
	namespaceParam = parameter{
		Id:          "NamespaceParam",
		SchemaType:  stringSchema,
		Description: "Filters results based on the specified namespace.",
		Name:        "namespace",
		In:          inQuery,
	}
	nextTokenParam = parameter{
		Id:          "NextTokenParam",
		SchemaType:  stringSchema,
		Description: "Indicates where to start paging for queries that support pagination.",
		Name:        "next_token",
		In:          inQuery,
	}
	perPageParam = parameter{
		Id:          "PerPageParam",
		SchemaType:  intSchema,
		Description: "Maximum number of results to return.",
		Name:        "per_page",
		In:          inQuery,
	}
	prefixParam = parameter{
		Id:          "PrefixParam",
		SchemaType:  stringSchema,
		Description: "Constrains results to jobs that start with the defined prefix",
		Name:        "prefix",
		In:          inQuery,
	}
	regionParam = parameter{
		Id:          "RegionParam",
		SchemaType:  stringSchema,
		Description: "Filters results based on the specified region.",
		Name:        "region",
		In:          inQuery,
	}
	snapshotIDParam = parameter{
		Id:          "SnapshotIDParam",
		SchemaType:  stringSchema,
		Description: "The ID of the snapshot to target.",
		Name:        "snapshot_id",
		In:          inQuery,
	}
	staleParam = parameter{
		Id:          "StaleParam",
		SchemaType:  stringSchema,
		Description: "If present, results will include stale reads.",
		Name:        "stale",
		In:          inQuery,
	}
	volumeActionParam = parameter{
		Id:          "VolumeActionParam",
		SchemaType:  stringSchema,
		Description: "The action to perform on the Volume (create, detach, delete).",
		Name:        "action",
		In:          inPath,
		Required:    true,
	}
	volumeForceParam = parameter{
		Id:          "VolumeForceParam",
		SchemaType:  stringSchema,
		Description: "Used to force the de-registration of a volume.",
		Name:        "force",
		In:          inQuery,
	}
	volumeIDParam = parameter{
		Id:          "VolumeIDParam",
		SchemaType:  stringSchema,
		Description: "Volume unique identifier.",
		Name:        "volumeId",
		In:          inPath,
		Required:    true,
	}
	volumeNodeParam = parameter{
		Id:          "VolumeNodeParam",
		SchemaType:  stringSchema,
		Description: "Specifies node to target volume operation for.",
		Name:        "node",
		In:          inQuery,
	}
	volumeNodeIDParam = parameter{
		Id:          "VolumeNodeIDParam",
		SchemaType:  stringSchema,
		Description: "Filters volume lists by node ID.",
		Name:        "node_id",
		In:          inQuery,
	}
	volumePluginIDParam = parameter{
		Id:          "VolumePluginIDParam",
		SchemaType:  stringSchema,
		Description: "Filters volume lists by plugin ID.",
		Name:        "plugin_id",
		In:          inQuery,
	}
	volumeTypeParam = parameter{
		Id:          "VolumeTypeParam",
		SchemaType:  stringSchema,
		Description: "Filters volume lists to a specific type.",
		Name:        "type",
		In:          inQuery,
	}
	waitParam = parameter{
		Id:          "WaitParam",
		SchemaType:  intSchema,
		Description: "Provided with IndexParam to wait for change.",
		Name:        "wait",
		In:          inQuery,
	}
	idempotencyTokenParam = parameter{
		Id:          "IdempotencyTokenParam",
		SchemaType:  stringSchema,
		Description: "Can be used to ensure operations are only run once.",
		Name:        "idempotency_token",
		In:          inQuery,
	}
	nomadTokenHeader = parameter{
		Id:          "NextTokenHeader",
		SchemaType:  stringSchema,
		Description: "A Nomad ACL token.",
		Name:        "X-Nomad-Token",
		In:          inHeader,
	}
)

var (
	xNomadIndexHeader = responseHeader{
		Name:        "X-Nomad-Index",
		SchemaType:  intSchema,
		Description: "A unique identifier representing the current state of the requested resource. On a new Nomad cluster the value of this index starts at 1.",
	}
	xNomadKnownLeaderHeader = responseHeader{
		Name:        "X-Nomad-KnownLeader",
		SchemaType:  boolSchema,
		Description: "Boolean indicating if there is a known cluster leader.",
	}
	xNomadLastContactHeader = responseHeader{
		Name:        "X-Nomad-LastContact",
		SchemaType:  intSchema,
		Description: "The time in milliseconds that a server was last contacted by the leader node.",
	}
)

var queryMeta = []*responseHeader{
	&xNomadIndexHeader,
	&xNomadKnownLeaderHeader,
	&xNomadLastContactHeader,
}

var writeMeta = []*responseHeader{
	&xNomadIndexHeader,
}

var queryOptions = []*parameter{
	&regionParam,
	&namespaceParam,
	&indexHeader,
	&waitParam,
	&staleParam,
	&prefixParam,
	&nomadTokenHeader,
	&perPageParam,
	&nextTokenParam,
}

var writeOptions = []*parameter{
	&regionParam,
	&namespaceParam,
	&nomadTokenHeader,
	&idempotencyTokenParam,
}

var (
	badRequestResponse = response{
		Name:        "BadRequestResponse",
		Description: "Bad request",
	}
	forbiddenResponse = response{
		Name:        "ForbiddenResponse",
		Description: "Forbidden",
	}
	internalServerErrorResponse = response{
		Name:        "InternalServerErrorResponse",
		Description: "Internal server error",
	}
	methodNotAllowedResponse = response{
		Name:        "MethodNotAllowedResponse",
		Description: "Method not allowed",
	}
	notFoundResponse = response{
		Name:        "NotFoundResponse",
		Description: "Not found",
	}
)

var standardResponses = []*responseConfig{
	{
		Code:     400,
		Response: &badRequestResponse,
	},
	{
		Code:     403,
		Response: &forbiddenResponse,
	},
	{
		Code:     405,
		Response: &methodNotAllowedResponse,
	},
	{
		Code:     500,
		Response: &internalServerErrorResponse,
	},
}

func newOperation(method string, handler string, tags []string, operationId string, requestBody *requestBody, params []*parameter, responses ...*responseConfig) *operation {
	return &operation{
		Method:      method,
		Handler:     handler,
		Tags:        tags,
		OperationId: operationId,
		RequestBody: requestBody,
		Parameters:  params,
		Responses:   getResponses(responses...),
	}
}

func newRequestBody(schemaType schemaType, model interface{}) *requestBody {
	return &requestBody{
		SchemaType: schemaType,
		Model:      reflect.TypeOf(model),
	}
}

func newResponseConfig(statusCode int, schemaType schemaType, model interface{}, headers []*responseHeader, name string) *responseConfig {
	cfg := &responseConfig{
		Code:    statusCode,
		Headers: headers,
		Response: &response{
			Name: name,
		},
	}

	if model != nil {
		cfg.Content = &content{
			SchemaType: schemaType,
			Model:      reflect.TypeOf(model),
		}
	}

	return cfg
}

func getResponses(configs ...*responseConfig) []*responseConfig {
	responses := append(standardResponses, configs...)
	return responses
}
