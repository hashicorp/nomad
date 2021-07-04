package main

import (
	"bytes"
	openapi3 "github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestNomadPathAdapterFunc(t *testing.T) {
	req := require.New(t)

	nsb := NewNomadSpecBuilder()
	nsb.spec = &Spec{}

	for _, adapter := range nsb.PathAdapters {
		err := adapter.Parser.Parse()
		req.NoError(err, "TestNomadPathAdapterFunc.nsb.PathAdapter.Parser.Parse")
	}

	for key, _ := range nsb.spec.Model.Paths {
		t.Log(key)
	}
	t.Log(nsb)
}

func TestRenderSchema(t *testing.T) {
	req := require.New(t)

	spec := Spec{
		OpenAPIVersion: "3.0.3",
		Model: openapi3.T{
			Components: openapi3.Components{
				Schemas: openapi3.Schemas{
					"Service": &openapi3.SchemaRef{
						Ref: "#/components/schemas/Service",
					},
				},
			},
		},
	}

	var formatted []byte
	var err error
	var buf bytes.Buffer
	var generator = Generator{
		OutputFile: "./",
		spec:       &spec,
	}

	err = generator.execTemplate(&buf, specTmpl)
	req.NoError(err, "TestRenderSchema.spec.execTemplate failed")

	formatted, err = generator.format(buf.Bytes())
	req.NoError(err, "TestRenderSchema.spec.format failed")

	req.Contains(formatted, serviceSchema)
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
