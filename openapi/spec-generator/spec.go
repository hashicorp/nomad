package main

import (
	"context"
	"fmt"
	openapi3 "github.com/getkin/kin-openapi/openapi3"
)

// Spec wraps a kin-openapi document object model with a little bit of extra
// metadata so that the template can be entirely data driven
type Spec struct {
	ValidationContext context.Context // Required parameter for validation functions
	OpenAPIVersion    string          // Required for template rendering
	Model             openapi3.T      // Document object model we are building
}

// AdapterFunc is an injectable behavior that is responsible for adapting the
// results of a parsing operation to a kin-openapi document object model.
type AdapterFunc func(*SpecBuilder, *ParseResult) error

// SourceAdapter allows the coupling of a PackageParser with a specific
// Adapt function that knows how to process the results of that parsing operation.
type SourceAdapter struct {
	Parser *PackageParser
	Adapt  AdapterFunc
}

// SpecBuilderExt allows the injection of implementation specific helper methods
// and behaviors.
type SpecBuilderExt interface {
	SpecBuilder() *SpecBuilder // Not strictly required but seems like a good practice
}

// SpecBuilder allows specifying different static analysis behaviors to that the
// framework can target any extant API.
type SpecBuilder struct {
	spec *Spec
	Ext  SpecBuilderExt // Allows injection of variable behavior per implementation.
	// PathAdapters are used to parse and adapt all code related to path handling.
	// They are represented as a slice since adapter logic may vary by package.
	PathAdapters []*SourceAdapter
	// ComponentAdapters are used to parse and adapt all code related to components.
	// They are represented as a slice since adapter logic may vary by package.
	ComponentAdapters []*SourceAdapter
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
		b.spec.Model.Security = openapi3.SecurityRequirements{}
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
	if err := b.processAdapters(b.ComponentAdapters, "BuildComponents"); err != nil {
		return err
	}

	return nil
}

// BuildPaths builds the Paths field
func (b *SpecBuilder) BuildPaths() error {
	if err := b.processAdapters(b.PathAdapters, "BuildPaths"); err != nil {
		return err
	}

	return nil
}

// Template method for parsing and adapting source code
func (b *SpecBuilder) processAdapters(adapters []*SourceAdapter, caller string) error {
	for _, adapter := range adapters {
		results, err := adapter.Parser.Parse()
		if err != nil {
			return fmt.Errorf("SpecBuilder.%s.Parser.Parse: %v\n", caller, err)
		}

		for _, result := range *results {
			if err := adapter.Adapt(b, result); err != nil {
				return fmt.Errorf("SpecBuilder.%s.adapter.AdapterFunc: %v\n", caller, err)
			}
		}
	}

	return nil
}
