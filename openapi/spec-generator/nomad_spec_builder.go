package main

import (
	"context"
	"github.com/getkin/kin-openapi/openapi3"
)

type NomadSpecBuilder struct {
	spec *Spec
}

func (nsb *NomadSpecBuilder) Build() (*Spec, error) {
	nsb.spec = &Spec{
		ValidationContext: context.Background(),
		OpenAPIVersion:    "3.0.3",
		Model:             openapi3.T{},
	}

	if err := nsb.BuildSecurity(); err != nil {
		return nil, err
	}

	if err := nsb.BuildServers(); err != nil {
		return nil, err
	}

	if err := nsb.BuildTags(); err != nil {
		return nil, err
	}

	if err := nsb.BuildComponents(); err != nil {
		return nil, err
	}

	if err := nsb.BuildPaths(); err != nil {
		return nil, err
	}

	if err := nsb.spec.Model.Validate(nsb.spec.ValidationContext); err != nil {
		return nil, err
	}

	return nsb.spec, nil
}

func (nsb *NomadSpecBuilder) BuildComponents() error {

	return nil
}

func (nsb *NomadSpecBuilder) BuildPaths() error {

	return nil
}

func (nsb *NomadSpecBuilder) BuildSecurity() error {

	return nil
}

func (nsb *NomadSpecBuilder) BuildServers() error {

	return nil
}

func (nsb *NomadSpecBuilder) BuildTags() error {

	return nil
}
