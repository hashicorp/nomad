package main

import (
	"context"
	openapi3 "github.com/getkin/kin-openapi/openapi3"
	"github.com/ghodss/yaml"
)

// Spec wraps a kin-openapi document object model with a little bit of extra
// metadata so that the template can be entirely data driven
type Spec struct {
	ValidationContext context.Context // Required parameter for validation functions
	OpenAPIVersion    string          // Required for template rendering
	Model             openapi3.T      // Document object model we are building
}

func (s *Spec) ToYAML() (string, error) {
	data, err := yaml.Marshal(s)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// SpecBuilderExt allows the injection of implementation specific helper methods
// and behaviors.
type SpecBuilderExt interface {
	SpecBuilder() *SpecBuilder // Not strictly required but seems like a good practice
}

// SpecBuilder allows specifying different static analysis behaviors to that the
// framework can target any extant API.
type SpecBuilder struct {
	spec    *Spec
	Visitor PackageVisitor
	Ext     SpecBuilderExt // Allows injection of variable behavior per implementation.
}

// Build runs a default implementation to build and OpenAPI spec. Derived types
// may choose to override if they don't need to execute this full pipeline.
func (b *SpecBuilder) Build() (*Spec, error) {
	// TODO: Eventually may need to support multiple OpenAPI versions, but pushing
	// that off for now.
	b.spec = &Spec{
		ValidationContext: context.Background(),
		OpenAPIVersion:    "3.0.3",
		Model:             openapi3.T{},
	}

	if err := b.BuildSecurity(); err != nil {
		return nil, err
	}

	if err := b.BuildServers(); err != nil {
		return nil, err
	}

	if err := b.BuildTags(); err != nil {
		return nil, err
	}

	if err := b.BuildComponents(); err != nil {
		return nil, err
	}

	if err := b.BuildPaths(); err != nil {
		return nil, err
	}

	if err := b.spec.Model.Validate(b.spec.ValidationContext); err != nil {
		return nil, err
	}

	return b.spec, nil
}

// BuildSecurity builds the Security field
// TODO: Might be useful for interface, but might not need this for Nomad
func (b *SpecBuilder) BuildSecurity() error {
	if b.spec.Model.Security == nil {
		b.spec.Model.Security = *openapi3.NewSecurityRequirements()
	}
	return nil
}

// BuildServers builds the Servers field
// TODO: Might be useful for interface, but might not need this for Nomad
func (b *SpecBuilder) BuildServers() error {
	if b.spec.Model.Servers == nil {
		b.spec.Model.Servers = openapi3.Servers{}
	}
	return nil
}

// BuildTags builds the Tags field
// TODO: Might be useful for interface, but might not need this for Nomad
func (b *SpecBuilder) BuildTags() error {
	if b.spec.Model.Tags == nil {
		b.spec.Model.Tags = openapi3.Tags{}
	}
	return nil
}

// BuildComponents builds the Components field
// TODO: Might be useful for interface, but might not need this for Nomad
func (b *SpecBuilder) BuildComponents() error {
	b.spec.Model.Components = openapi3.NewComponents()

	b.spec.Model.Components.Schemas = b.Visitor.SchemaRefs()
	b.spec.Model.Components.Parameters = b.Visitor.ParameterRefs()
	b.spec.Model.Components.Headers = b.Visitor.HeaderRefs()
	b.spec.Model.Components.RequestBodies = b.Visitor.RequestBodyRefs()
	b.spec.Model.Components.Callbacks = b.Visitor.CallbackRefs()
	b.spec.Model.Components.Responses = b.Visitor.ResponseRefs()
	b.spec.Model.Components.SecuritySchemes = openapi3.SecuritySchemes{}

	return nil
}

// BuildPaths builds the Paths field
func (b *SpecBuilder) BuildPaths() error {
	if b.spec.Model.Paths == nil {
		b.spec.Model.Paths = openapi3.Paths{}
	}

	for _, adapter := range b.Visitor.HandlerAdapters() {
		pathItem := &openapi3.PathItem{}

		b.spec.Model.Paths[adapter.GetPath()] = pathItem
	}

	return nil
}
