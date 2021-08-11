package main

import (
	"context"
	"os"
	"path"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/require"
)

func TestGenSchema(t *testing.T) {
	dir := t.TempDir()
	outputPath := path.Join(dir, "test-gen-schema.yaml")

	exitCode := run(outputPath)
	require.Equal(t, 0, exitCode)
	_, err := os.Stat(outputPath)
	require.NoError(t, err)

	loader := openapi3.NewLoader()
	var model *openapi3.T
	model, err = loader.LoadFromFile(outputPath)
	require.NoError(t, err)
	require.NotNil(t, model)

	err = model.Validate(context.Background())
	require.NoError(t, err)
}
