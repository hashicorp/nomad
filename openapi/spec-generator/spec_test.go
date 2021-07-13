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
	debugOptions.printHelpers = true
	debugOptions.printReturnSource = true
	debugOptions.printSchemaRefs = true
	debugOptions.filterByMethods = []string{"agent.jobListRequest"}
	var analyzer *Analyzer
	var err error

	analyzer, err = NewAnalyzer(nomadPackages, t.Log, debugOptions)
	req.NoError(err)

	visitor := NewNomadPackageVisitor(analyzer, t.Log, debugOptions)

	parser := PackageParser{
		Visitor: &visitor,
	}

	err = parser.Parse()
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
	//req.Contains(yaml, serviceSchema)
}

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
