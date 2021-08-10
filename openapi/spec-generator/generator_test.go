package spec

import (
	"context"
	"os"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/require"
)

func TestGenSchema(t *testing.T) {
	generator := Generator{
		logger:     t.Log,
		outputPath: "./test-gen-schema.yaml",
	}

	// TODO: This is currently only here for manual spec generation. It will
	// be removed as we move to CI based generation.
	// generator.outputPath = "../v1/openapi.yaml"

	err := generator.run()
	require.NoError(t, err, "Generator.run failed")
	_, err = os.Stat(generator.outputPath)
	require.NoError(t, err)

	loader := openapi3.NewLoader()
	var model *openapi3.T
	model, err = loader.LoadFromFile(generator.outputPath)
	require.NoError(t, err, "LoadFromFile failed")
	require.NotNil(t, model)

	t.Run("nested", func(t *testing.T) {
		require.NoError(t, err)
	})

	err = model.Validate(context.Background())
	require.NoError(err, "Model Validation failed")

	// Comment these lines when manually generating a spec prior to CI based generation.
	err = os.Remove(generator.outputPath)
	require.NoError(err)
}
