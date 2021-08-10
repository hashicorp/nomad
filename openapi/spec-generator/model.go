package spec

import (
	"reflect"
)

const (
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

const (
	inHeader = "header"
	inQuery  = "query"
	inPath   = "path"
	inCookie = "cookie"
)

type ResponseHeader struct {
	Name        string
	SchemaType  string
	Description string
}

type RequestBody struct {
	SchemaType string
	Model      reflect.Type
}
type Content struct {
	SchemaType string
	Model      reflect.Type
}

type Response struct {
	Name        string
	Description string
}

type ResponseConfig struct {
	Code     int
	Response *Response
	Content  *Content
	Headers  []*ResponseHeader
}

type Operation struct {
	Method      string
	Handler     string
	Tags        []string
	OperationId string
	Summary     string
	Description string
	RequestBody *RequestBody
	Parameters  []*Parameter
	Responses   []*ResponseConfig
}

type Path struct {
	Template   string
	Operations []*Operation
}
