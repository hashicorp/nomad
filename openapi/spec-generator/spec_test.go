package main

import (
	"context"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

func TestLoadSpec(t *testing.T) {
	req := require.New(t)
	loader := openapi3.Loader{}
	spec, err := loader.LoadFromFile("job-run.yaml")
	req.NoError(err)
	req.NotNil(spec)

	err = spec.Validate(context.Background())
	req.NoError(err)
}

func TestRenderSchema(t *testing.T) {
	req := require.New(t)

	debugOptions := defaultDebugOptions
	debugOptions.printSource = false
	debugOptions.printHandlers = false
	debugOptions.printVariables = false

	var analyzer *Analyzer
	var err error

	analyzer, err = newAnalyzer(nomadPackages, t.Log, debugOptions)
	req.NoError(err)

	visitor := newNomadPackageVisitor(analyzer, t.Log, debugOptions)

	err = visitor.Parse()
	req.NoError(err, "TestPackageVisitor.parser.Parse")
	adapterCount := len(visitor.HandlerAdapters())
	t.Log("adapterCount", adapterCount)
	req.NotEqual(0, adapterCount)

	builder := newNomadSpecBuilder(analyzer, &visitor, t.Log)
	var spec *Spec
	spec, err = builder.Build()
	req.NoError(err)
	req.NotNil(spec)
	req.NotNil(spec.Model)
	req.NotNil(spec.Model.Components)

	var yaml string
	yaml, err = spec.ToYAML()
	_ = os.WriteFile("spec_test.yaml", []byte(yaml), 0666)
	t.Log(yaml)
	req.NotEmpty(yaml)
	req.Contains(yaml, jobListStubSchema)
}

func TestJobRequest(t *testing.T) {
	req := require.New(t)

	debugOptions := defaultDebugOptions
	debugOptions.printSource = false
	debugOptions.printHandlers = false
	debugOptions.printVariables = false
	debugOptions.filterByMethods = []string{"agent.jobUpdate"}
	debugOptions.additionalSchemas = []string{"api.JobRegisterRequest"}

	var analyzer *Analyzer
	var err error

	analyzer, err = newAnalyzer(nomadPackages, t.Log, debugOptions)
	req.NoError(err)

	visitor := newNomadPackageVisitor(analyzer, t.Log, debugOptions)

	err = visitor.Parse()
	req.NoError(err, "TestJobRequest.parser.Parse")
	adapterCount := len(visitor.HandlerAdapters())
	t.Log("adapterCount", adapterCount)
	req.NotEqual(0, adapterCount)

	builder := newNomadSpecBuilder(analyzer, &visitor, t.Log)
	var spec *Spec
	spec, err = builder.Build()
	req.NoError(err)
	req.NotNil(spec)
	req.NotNil(spec.Model)
	req.NotNil(spec.Model.Components)

	var yaml string
	yaml, err = spec.ToYAML()
	_ = os.WriteFile("jobRequest.yaml", []byte(yaml), 0666)
	t.Log(yaml)
	req.NotEmpty(yaml)
	req.Contains(yaml, jobResponseSchema)
}

func TestBuildFromSchema(t *testing.T) {
	req := require.New(t)

	var analyzer *Analyzer
	var err error

	analyzer, err = newAnalyzer(nomadPackages, t.Log, defaultDebugOptions)
	req.NoError(err)

	visitor := newNomadPackageVisitor(analyzer, t.Log, defaultDebugOptions)

	builder := newNomadSpecBuilder(analyzer, &visitor, t.Log)
	var spec *Spec
	spec, err = builder.BuildFromModel()
	req.NoError(err)
	req.NotNil(spec)
	req.NotNil(spec.Model)
	req.NotNil(spec.Model.Components)

	var yaml string
	yaml, err = spec.ToYAML()
	_ = os.WriteFile("from-model.yaml", []byte(yaml), 0666)
	t.Log(yaml)
	req.NotEmpty(yaml)
	req.Contains(yaml, jobResponseSchema)
}

var jobResponseSchema = `
    JobRegisterResponse:
      properties:
        EvalCreateIndex:
          maximum: 1.8446744073709552e+19
          minimum: 0
          type: integer
        EvalID:
          type: string
        JobModifyIndex:
          maximum: 1.8446744073709552e+19
          minimum: 0
          type: integer
        QueryMeta:
          $ref: '#/components/schemas/QueryMeta'
        Warnings:
          type: string
      type: object
`

var jobListStubSchema = `
    JobListStub:
      properties:
        CreateIndex:
          type: integer
          format: uint64
        Datacenters:
          type: array
          items:
            type: string
        ID:
          type: string
        JobModifyIndex:
          type: integer
          format: uint64
        JobSummary:
          $ref: '#/components/schemas/JobSummary'
        ModifyIndex:
          type: integer
          format: uint64
        Name:
          type: string
        Namespace:
          type: string
        ParameterizedJob:
          type: boolean
        ParentID:
          type: string
        Periodic:
          type: boolean
        Priority:
          type: integer
          format: int64
        Status:
          type: string
        StatusDescription:
          type: string
        Stop:
          type: boolean
        SubmitTime:
          type: integer
          format: int64
        Type:
          type: string
      type: object
      x-go-package: github.com/hashicorp/nomad/api
`

var serviceSchema = `
Service:
      type: object
      title: Service represents a Consul service definition.
      properties:
        AddressMode:
          type: string
        CanaryMeta:
          type: object
          additionalProperties:
            type: string
        CanaryTags:
          type: array
          items:
            type: string
        CheckRestart:
          $ref: '#/components/schemas/CheckRestart'
        Checks:
          type: array
          items:
            $ref: '#/components/schemas/ServiceCheck'
        Connect:
          $ref: '#/components/schemas/ConsulConnect'
        EnableTagOverride:
          type: boolean
        Id:
          description: FIXME Id is unused. Remove?
          type: string
        Meta:
          type: object
          additionalProperties:
            type: string
        Name:
          type: string
        OnUpdate:
          type: string
        PortLabel:
          type: string
        Tags:
          type: array
          items:
            type: string
        TaskName:
          type: string
      x-go-package: github.com/hashicorp/nomad/api
`
