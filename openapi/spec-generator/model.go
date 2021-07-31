package main

import (
	"github.com/hashicorp/nomad/api"
	"net/http"
	"reflect"
)

var (
	arraySchema  = "array"
	objectSchema = "object"
	stringSchema = "string"
	numberSchema = "number"
	boolSchema   = "boolean"
	intSchema    = "integer"
)

type Parameter struct {
	Id          string
	Name        string
	SchemaType  string
	In          string
	Description string
	Required    bool
}

var (
	inHeader = "header"
	inQuery  = "query"
	inPath   = "path"
	inCookie = "cookie"
)

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

type ResponseHeader struct {
	Name        string
	SchemaType  string
	Description string
}

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

type RequestBody struct {
	SchemaType string
	Model      reflect.Type
}
type ResponseContent struct {
	SchemaType string
	Model      reflect.Type
}

type Response struct {
	Id          string
	Description string
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

type ResponseConfig struct {
	Code     int
	Response *Response
	Content  *ResponseContent
	Headers  []*ResponseHeader
}

type Operation struct {
	Method      string
	Tags        []string
	OperationId string
	Summary     string
	Description string
	RequestBody *RequestBody
	Parameters  []*Parameter
	Responses   []*ResponseConfig
}

type Path struct {
	Key        string
	Operations []*Operation
}

type V1API struct{}

func (v *V1API) GetPaths() []*Path {
	return []*Path{
		{
			Key: "/jobs/{jobName}",
			Operations: []*Operation{
				NewOperation(http.MethodGet, []string{"Jobs"}, "getJob",
					nil,
					append(queryOptions, &JobNameParam),
					NewResponseConfig(200, objectSchema, api.Job{}, queryMeta, "GetJobResponse"),
				),
			},
		},
		{
			Key: "/jobs",
			Operations: []*Operation{
				NewOperation(http.MethodPost, []string{"Jobs"}, "postJob",
					NewRequestBody(objectSchema, api.JobRegisterRequest{}),
					nil,
					NewResponseConfig(200, objectSchema, api.JobRegisterResponse{}, queryMeta, "PostJobResponse"),
				),
			},
		},
		{
			Key: "/job/{jobName}/plan",
			Operations: []*Operation{
				NewOperation(http.MethodPost, []string{"Jobs"}, "postJobPlan",
					NewRequestBody(objectSchema, api.JobPlanRequest{}),
					append(queryOptions, &JobNameParam),
					NewResponseConfig(200, objectSchema, api.JobPlanResponse{}, queryMeta, "PostJobPlanResponse"),
				),
			},
		},
	}
}

func NewOperation(method string, tags []string, operationId string, requestBody *RequestBody, params []*Parameter, responses ...*ResponseConfig) *Operation {
	return &Operation{
		Method:      method,
		Tags:        tags,
		OperationId: operationId,
		RequestBody: requestBody,
		Parameters:  params,
		Responses:   getResponses(responses...),
	}
}

func NewRequestBody(schemaType string, model interface{}) *RequestBody {
	return &RequestBody{
		SchemaType: schemaType,
		Model:      reflect.TypeOf(model),
	}
}

func NewResponseConfig(statusCode int, schemaType string, model interface{}, headers []*ResponseHeader, id string) *ResponseConfig {
	return &ResponseConfig{
		Code: statusCode,
		Content: &ResponseContent{
			SchemaType: schemaType,
			Model:      reflect.TypeOf(model),
		},
		Headers: headers,
		Response: &Response{
			Id: id,
		},
	}
}

func getResponses(configs ...*ResponseConfig) []*ResponseConfig {
	responses := append(standardResponses, configs...)
	return responses
}
