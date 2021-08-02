package main

import (
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

type ResponseHeader struct {
	Name        string
	SchemaType  string
	Description string
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

func newOperation(method string, tags []string, operationId string, requestBody *RequestBody, params []*Parameter, responses ...*ResponseConfig) *Operation {
	return &Operation{
		Method:      method,
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

func newResponseConfig(statusCode int, schemaType string, model interface{}, headers []*ResponseHeader, id string) *ResponseConfig {
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
