package main

import (
	"reflect"
)

type schemaType string

const (
	arraySchema  schemaType = "array"
	objectSchema            = "object"
	stringSchema            = "string"
	numberSchema            = "number"
	boolSchema              = "boolean"
	intSchema               = "integer"
	nilSchema               = ""
)

type parameter struct {
	Id          string
	Name        string
	SchemaType  schemaType
	In          parameterLocation
	Description string
	Required    bool
}

type parameterLocation string

const (
	inHeader parameterLocation = "header"
	inQuery                    = "query"
	inPath                     = "path"
	inCookie                   = "cookie"
)

type responseHeader struct {
	Name        string
	SchemaType  schemaType
	Description string
}

type requestBody struct {
	SchemaType schemaType
	Model      reflect.Type
}

type content struct {
	SchemaType schemaType
	Model      reflect.Type
}

type response struct {
	Name        string
	Description string
}

type responseConfig struct {
	Code     int
	Response *response
	Content  *content
	Headers  []*responseHeader
}

type operation struct {
	Method      string
	Handler     string
	Tags        []string
	OperationId string
	Summary     string
	Description string
	RequestBody *requestBody
	Parameters  []*parameter
	Responses   []*responseConfig
}

type apiPath struct {
	Template   string
	Operations []*operation
}
