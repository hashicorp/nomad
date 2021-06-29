package main

import (
	"bytes"
	openapi3 "github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestRenderSchema(t *testing.T) {
	req := require.New(t)

	spec := Spec{
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
	err = spec.execTemplate(&buf, specTmpl)
	req.NoError(err, "TestRenderSchema.spec.execTemplate failed")

	formatted, err = spec.format(buf.Bytes())
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
