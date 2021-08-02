package main

import (
	"context"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

func TestGenSchema(t *testing.T) {
	req := require.New(t)

	generator := Generator{
		logger: t.Log,
	}

	err := generator.run()
	req.NoError(err, "Generator.run failed")
	_, err = os.Stat("../v1/openapi.yaml")
	req.NoError(err)

	loader := openapi3.NewLoader()
	var model *openapi3.T
	model, err = loader.LoadFromFile("../v1/openapi.yaml")
	req.NoError(err, "LoadFromFile failed")
	req.NotNil(model)

	err = model.Validate(context.Background())
	req.NoError(err, "Model Validation failed")
}
