package main

import (
	"fmt"
	"github.com/hashicorp/go-hclog"
	"os"
)

// Generator iterates over the input source and configuration, and aggregates a
// data model a that can be used to render an openapi from the template.
type Generator struct {
	logger loggerFunc
}

type loggerFunc func(args ...interface{})

func (g *Generator) log(args ...interface{}) {
	hclog.Default().Log(hclog.Info, "OpenAPI Spec Generator:", args)
}

// init outputs an OpenAPI spec from the Spec model and template file.
func (g *Generator) run() error {
	if g.logger == nil {
		g.logger = g.log
	}

	builder := SpecBuilder{}
	spec, err := builder.BuildFromModel(g.logger)
	if err != nil {
		return err
	}

	var yaml []byte
	yaml, err = spec.ToBytes()

	if err = os.WriteFile("../v1/openapi.yaml", yaml, 0644); err != nil {
		return fmt.Errorf("Generator.run.os.WriteFile: %s\n", err)
	}

	return nil
}
