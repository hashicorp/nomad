package main

import (
	"context"
	"fmt"
	openapi3 "github.com/getkin/kin-openapi/openapi3"
)

// Spec wraps a kin-openapi document object model with a little bit of extra
// metadata so that the template can be entirely data driven
type Spec struct {
	// TODO: Find a better solution to making this functionality available.
	analyzer          *Analyzer
	ValidationContext context.Context
	OpenAPIVersion    string
	Model             openapi3.T
}

type AdapterFunc func(*Spec, *ParseResult) error

type SourceAdapter struct {
	Parser *PackageParser
	Adapt  AdapterFunc
}

// SpecBuilder allows specifying different static analysis behaviors to that the
// framework can target any extant API.
type SpecBuilder struct {
	spec              *Spec
	PathAdapters      []*SourceAdapter // Used parse and adapt all code related to path handling
	ComponentAdapters []*SourceAdapter // Used parse and adapt all code related to components
}

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

func (b *SpecBuilder) BuildSecurity() error {

	return nil
}

func (b *SpecBuilder) BuildServers() error {

	return nil
}

func (b *SpecBuilder) BuildTags() error {

	return nil
}

func (b *SpecBuilder) BuildComponents() error {
	if err := b.processAdapters(b.ComponentAdapters, "BuildComponents"); err != nil {
		return err
	}

	return nil
}

func (b *SpecBuilder) BuildPaths() error {
	if err := b.processAdapters(b.PathAdapters, "BuildPaths"); err != nil {
		return err
	}

	return nil
}

func (b *SpecBuilder) processAdapters(adapters []*SourceAdapter, caller string) error {
	for _, adapter := range adapters {
		results, err := adapter.Parser.Parse()
		if err != nil {
			return fmt.Errorf("SpecBuilder.%s.Parser.Parse: %v\n", caller, err)
		}

		for _, result := range *results {
			if err := adapter.Adapt(b.spec, result); err != nil {
				return fmt.Errorf("SpecBuilder.%s.adapter.AdapterFunc: %v\n", caller, err)
			}
		}
	}

	return nil
}
