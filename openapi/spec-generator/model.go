package main

import (
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/nomad/structs"
	"net/http"
	"reflect"
)

type SchemaType string

var (
	arrayType  SchemaType = "array"
	objectType SchemaType = "object"
	stringType SchemaType = "string"
	numberType SchemaType = "number"
	boolType   SchemaType = "boolean"
	intType    SchemaType = "integer"
)

type Header struct {
	Name        string
	SchemaType  SchemaType
	Description string
}

var (
	IndexHeader = Header{
		Name: "X-Nomad-Index",
		SchemaType: intType,
	}

	KnownLeaderHeader = Header{
		Name: "X-Nomad-Known-Leader",
		SchemaType: boolType,
	}

	LastContactHeader = Header {
		Name: "X-Nomad-Last-Contact",
		SchemaType: intType,
	}
)

type Parameter struct {
	Name        string
	Type        SchemaType
	In          string
	Description string
}

var (
	AllParam:
schema:
type: integer
description: Flag indicating whether to constrain by job creation index or not.
name: all
in: query
IndexParam:
schema:
type: integer
description: >-
If set, wait until query exceeds given index. Must be provided with
WaitParam.
name: index
in: query
JobNameParam:
schema:
type: string
description: The job identifier.
name: jobName
in: path
required: true
NamespaceParam:
schema:
type: string
description: Filters results based on the specified namespace
name: namespace
in: query
NextTokenParam:
schema:
type: string
description: Indicates where to start paging for queries that support pagination
name: next_token
in: query
PerPageParam:
schema:
type: integer
description: Maximum number of results to return
name: per_page
in: query
PrefixParam:
schema:
type: string
description: Constrains results to jobs that start with the defined prefix
name: prefix
in: query
RegionParam:
schema:
type: string
description: Filters results based on the specified region
name: region
in: query
StaleParam:
schema:
type: string
description: If present, results will include stale reads
name: stale
in: query
WaitParam:
schema:
type: integer
description: Provided with IndexParam to wait for change
name: wait
in: query
NomadTokenHeader:
schema:
type: string
description: A Nomad ACL token
name: X-Nomad-Token
in: header
)

type RequestBody struct {
	Type SchemaType
	Model reflect.Type
}
type ResponseContent struct {
	Type  SchemaType
	Model reflect.Type
}

type Response struct {
	Name    string
}

// TODO: Shold I just build the Kin objects
var (
	BadRequest:
description: Bad request
Forbidden:
description: Forbidden
InternalServerError:
description: Internal server error
MethodNotAllowed:
description: Method not allowed
NotFound:
description: Not found
)

type ResponseConfig struct {
	Code     int
	Response *Response
	Content ResponseContent
	Headers []Header
}

type PathItem struct {
	Method      string
	Tags        []string
	OperationId string
	Summary     string
	Description string
	RequestBody RequestBody
	Headers     []Header
	Parameters  []Parameter
	Responses   []ResponseConfig
}

type Path struct {
	Key   string
	Items []PathItem
}

func GetPaths() []Path {
	return []Path{
		Path{
			Key: "/v1/jobs",
			Items: []PathItem{
				PathItem{
					Method:        http.MethodGet,
					Tags: []string{"Jobs"},
					OperationId: "getJob",
					RequestBody: RequestBody{
						Type: objectType,
						Model: reflect.TypeOf(structs.JobSpecificRequest{}),
					},
					Parameters: []Parameter{RegionParam, NamespaceParam, TokenHeaderParam},
					Responses: []ResponseConfig{
						ResponseConfig{
							Code: 200,
							Content: ResponseContent{
								Type: objectType,
								Model: reflect.TypeOf(api.Job{}),
							},
							Headers: []Header{IndexHeader, KnownLeaderHeader, LastContactHeader},
						},
					},
				},
			},
		},
	}
}
