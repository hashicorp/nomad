package main

type Spec struct {
	SpecVersion string
	Info        Info
	Servers     []Server
	Components  Components
	Paths       []Path
}

type Info struct {
	Title          string
	Description    string
	TermsOfService string
	Contact        Contact
	License        License
	Version        string
}

type Contact struct {
	Name  string
	URL   string
	Email string
}

type License struct {
	Name string
	URL  string
}

type Server struct {
	URL         string
	Description string
	Variables   []ServerVariable
}

type ServerVariable struct {
	Name        string
	Enum        []string
	Default     string
	Description string
}

type Components struct {
	Schemas         []Schema
	Responses       []Response
	Parameters      []Parameter
	Examples        []Example
	RequestBodies   []RequestBody
	Headers         []Header
	SecuritySchemes SecurityScheme
	Links           []Link
	Callbacks       []Callback
}

type SecurityScheme struct {
}

type ParameterType string

const (
	InHeader ParameterType = "header"
	InQuery  ParameterType = "query"
	InPath   ParameterType = "Path"
)

type Parameter struct {
	Type ParameterType
}

type Header struct {
}

type Response struct {
}

type Schema struct {
}

type Example struct {
}

type Link struct {
}

type Callback struct {
}

type RequestBody struct {
}

type Path struct {
}
