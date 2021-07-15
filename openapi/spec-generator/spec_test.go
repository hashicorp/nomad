package main

import (
	"github.com/stretchr/testify/require"
	"testing"
)

//func TestNomadPathAdapterFunc(t *testing.T) {
//	req := require.New(t)
//
//	analyzer, err := NewAnalyzer(nomadPackages, t.Log, defaultDebugOptions)
//	req.NoError(err)
//
//	nsb := NewNomadSpecBuilder(analyzer)
//	nsb.spec = &Spec{}
//
//	for key, _ := range nsb.spec.Model.Paths {
//		t.Log(key)
//	}
//	t.Log(nsb)
//}

func TestRenderSchema(t *testing.T) {
	req := require.New(t)

	debugOptions := defaultDebugOptions
	debugOptions.printSource = false
	debugOptions.printHandlers = false
	debugOptions.printVariables = false
	// debugOptions.printHelpers = true
	// debugOptions.printReturnSource = true
	// debugOptions.printSchemaRefs = true
	// debugOptions.filterByMethods = []string{"agent.jobListRequest"}
	var analyzer *Analyzer
	var err error

	analyzer, err = NewAnalyzer(nomadPackages, t.Log, debugOptions)
	req.NoError(err)

	visitor := newNomadPackageVisitor(analyzer, t.Log, debugOptions)

	err = visitor.Parse()
	req.NoError(err, "TestPackageVisitor.parser.Parse")

	builder := NewNomadSpecBuilder(analyzer, &visitor)
	var spec *Spec
	spec, err = builder.Build()
	req.NoError(err)
	req.NotNil(spec)

	var yaml string
	yaml, err = spec.ToYAML()
	t.Log(yaml)
	req.NotEmpty(yaml)
	req.Contains(yaml, jobListStubSchema)
}

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
