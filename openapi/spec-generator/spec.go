package main

import (
	"context"
	openapi3 "github.com/getkin/kin-openapi/openapi3"
)

// Spec wraps a kin-openapi document object model with a little bit of extra
// metadata so that the template can be entirely data driven
type Spec struct {
	ValidationContext context.Context
	OpenAPIVersion    string
	Model             openapi3.T
}

// SpecBuilder allows specifying different static analysis behaviors to that the
// framework can target any extant API.
type SpecBuilder interface {
	Build() (*Spec, error)
	BuildComponents() error
	BuildPaths() error
	BuildSecurity() error
	BuildServers() error
	BuildTags() error // TODO: Not sure if this is needed.
}
