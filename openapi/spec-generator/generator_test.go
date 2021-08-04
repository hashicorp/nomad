package main

import (
	"context"
	"os"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/require"
)

func TestGenSchema(t *testing.T) {
	req := require.New(t)

	generator := Generator{
		logger:     t.Log,
		outputPath: "./test-gen-schema.yaml",
	}

	// TODO: This is currently only here for manual spec generation. It will
	// be removed as we move to CI based generation.
	// generator.outputPath = "../v1/openapi.yaml"

	err := generator.run()
	req.NoError(err, "Generator.run failed")
	_, err = os.Stat(generator.outputPath)
	req.NoError(err)

	loader := openapi3.NewLoader()
	var model *openapi3.T
	model, err = loader.LoadFromFile(generator.outputPath)
	req.NoError(err, "LoadFromFile failed")
	req.NotNil(model)

	err = model.Validate(context.Background())
	req.NoError(err, "Model Validation failed")

	// Comment these lines when manually generating a spec prior to CI based generation.
	err = os.Remove(generator.outputPath)
	req.NoError(err)
}
