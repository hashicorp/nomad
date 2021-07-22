package main

import (
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/nomad/structs"
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
	Name        string
	SchemaType  string
	In          string
	Description string
}

var (
	inHeader = "header"
	inQuery  = "query"
	inPath   = "path"
	inCookie = "cookie"
)

var (
	AllParam = Parameter{
		SchemaType:  intSchema,
		Description: "Flag indicating whether to constrain by job creation index or not.",
		Name:        "all",
		In:          inQuery,
	}
	IndexParam = Parameter{
		SchemaType:  intSchema,
		Description: "If set, wait until query exceeds given index. Must be provided with WaitParam.",
		Name:        "index",
		In:          inHeader,
	}
	JobNameParam = Parameter{
		SchemaType:  stringSchema,
		Description: "The job identifier.",
		Name:        "jobName",
		In:          inPath,
	}
	NamespaceParam = Parameter{
		SchemaType:  stringSchema,
		Description: "Filters results based on the specified namespace.",
		Name:        "namespace",
		In:          inQuery,
	}
	NextTokenParam = Parameter{
		SchemaType:  stringSchema,
		Description: "Indicates where to start paging for queries that support pagination.",
		Name:        "next_token",
		In:          inQuery,
	}
	PerPageParam = Parameter{
		SchemaType:  intSchema,
		Description: "Maximum number of results to return.",
		Name:        "per_page",
		In:          inQuery,
	}
	PrefixParam = Parameter{
		SchemaType:  stringSchema,
		Description: "Constrains results to jobs that start with the defined prefix",
		Name:        "prefix",
		In:          inQuery,
	}
	RegionParam = Parameter{
		SchemaType:  stringSchema,
		Description: "Filters results based on the specified region.",
		Name:        "region",
		In:          inQuery,
	}
	StaleParam = Parameter{
		SchemaType:  stringSchema,
		Description: "If present, results will include stale reads.",
		Name:        "stale",
		In:          inQuery,
	}
	WaitParam = Parameter{
		SchemaType:  intSchema,
		Description: "Provided with IndexParam to wait for change.",
		Name:        "wait",
		In:          inQuery,
	}
	NomadTokenParam = Parameter{
		SchemaType:  stringSchema,
		Description: "A Nomad ACL token.",
		Name:        "X-Nomad-Token",
		In:          inHeader,
	}
	KnownLeaderParam = Parameter{
		Name:        "X-Nomad-Known-Leader",
		SchemaType:  boolSchema,
		Description: "",
		In:          inHeader,
	}
	LastContactParam = Parameter{
		Name:        "X-Nomad-Last-Contact",
		SchemaType:  intSchema,
		Description: "",
		In:          inHeader,
	}
)

var queryMeta = []*Parameter{
	&IndexParam,
	&KnownLeaderParam,
	&LastContactParam,
}

var writeMeta = []*Parameter{
	&IndexParam,
}

var queryOptions = []*Parameter{
	&RegionParam,
	&NamespaceParam,
	&IndexParam,
	&WaitParam,
	&StaleParam,
	&PrefixParam,
	&NomadTokenParam,
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
	Name        string
	Description string
}

var (
	BadRequestRespnse = Response{
		Name:        "BadRequest",
		Description: "Bad request",
	}
	ForbiddenResponse = Response{
		Name:        "Forbidden",
		Description: "Forbidden",
	}
	InternalServerErrorResponse = Response{
		Name:        "InternalServerError",
		Description: "Internal server error",
	}
	MethodNotAllowedResponse = Response{
		Name:        "MethodNotAllowed",
		Description: "EMethod not allowed",
	}
	NotFoundResponse = Response{
		Name:        "NotFound",
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
	Headers  []*Parameter
}

type PathOperation struct {
	Method      string
	Tags        []string
	OperationId string
	Summary     string
	Description string
	RequestBody *RequestBody
	Headers     []*Parameter
	Parameters  []*Parameter
	Responses   []*ResponseConfig
}

type Path struct {
	Key        string
	Operations []*PathOperation
}

type V1API struct{}

func (v *V1API) GetPaths() []*Path {
	return []*Path{
		{
			Key: "/v1/jobs",
			Operations: []*PathOperation{
				NewPathItem(http.MethodGet, []string{"Jobs"}, "getJob",
					&RequestBody{
						SchemaType: objectSchema,
						Model:      reflect.TypeOf(structs.JobSpecificRequest{}),
					},
					queryOptions,
					&ResponseConfig{
						Code: 200,
						Content: &ResponseContent{
							SchemaType: objectSchema,
							Model:      reflect.TypeOf(api.Job{}),
						},
						Headers: queryMeta,
					}),
				NewPathItem(http.MethodPost, []string{"Jobs"}, "postJob",
					&RequestBody{
						SchemaType: objectSchema,
						Model:      reflect.TypeOf(api.JobRegisterRequest{}),
					},
					[]*Parameter{&JobNameParam},
					&ResponseConfig{
						Code: 200,
						Content: &ResponseContent{
							SchemaType: objectSchema,
							Model:      reflect.TypeOf(api.Job{}),
						},
						Headers: queryMeta,
					}),
			},
		},
	}
}

func NewPathItem(method string, tags []string, operationId string, requestBody *RequestBody, params []*Parameter, responses ...*ResponseConfig) *PathOperation {
	return &PathOperation{
		Method:      method,
		Tags:        tags,
		OperationId: operationId,
		RequestBody: requestBody,
		Parameters:  params,
		Responses:   getResponses(responses...),
	}
}

func getResponses(configs ...*ResponseConfig) []*ResponseConfig {
	return append(standardResponses, configs...)
}
