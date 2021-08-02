package main

import (
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

func TestBuildFromModel(t *testing.T) {
	req := require.New(t)

	var err error

	builder := SpecBuilder{}
	var spec *Spec
	spec, err = builder.BuildFromModel(t.Log)
	req.NoError(err)
	req.NotNil(spec)
	req.NotNil(spec.Model)
	req.NotNil(spec.Model.Components)

	var yamlBytes []byte
	yamlBytes, err = spec.ToBytes()
	yaml := string(yamlBytes)

	_ = os.WriteFile("test-build-from-model.yaml", yamlBytes, 0644)
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
        KnownLeader:
          type: boolean
        LastContact:
          format: int64
          type: integer
        LastIndex:
          maximum: 1.8446744073709552e+19
          minimum: 0
          type: integer
        RequestTime:
          format: int64
          type: integer
        Warnings:
          type: string
      type: object
`
