package main

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"go/format"
	"io"
	"os"
	"text/template"
)

//go:embed spec.tmpl
var specTmpl embed.FS

//go:embed param.tmpl
var paramTmpl embed.FS

//go:embed header.tmpl
var headerTmpl embed.FS

//go:embed response.tmpl
var responseTmpl embed.FS

//go:embed schema.tmpl
var schemaTmpl embed.FS

//go:embed paths.tmpl
var pathsTmpl embed.FS

// Generator iterates over the input source and configuration, and aggregates a
// data model a that can be used to render an openapi from the template.
type Generator struct {
	spec       *Spec
	OutputFile string
}

// RenderTemplate outputs an OpenAPI spec from the Spec model and template file.
func (g *Generator) RenderTemplate() error {
	if g.spec == nil {
		return errors.New("Generator.RenderTemplate: spec cannot be nil")
	}

	if err := g.spec.Model.Validate(g.spec.ValidationContext); err != nil {
		return fmt.Errorf("Generator.RenderTemplate.spec.Model.Validate: %v\n", err)
	}

	var formatted []byte
	var err error
	var buf bytes.Buffer

	if err = g.execTemplate(&buf, specTmpl); err != nil {
		return err
	}

	if formatted, err = g.format(buf.Bytes()); err != nil {
		return err
	}

	if err = os.WriteFile(g.OutputFile, formatted, 0644); err != nil {
		return fmt.Errorf("Generator.RenderTemplate.os.WriteFile: %s\n", err)
	}

	return nil
}

func (g *Generator) execTemplate(w io.Writer, file embed.FS) error {
	tmpl, err := template.ParseFS(file, "*")
	if err != nil {
		return errors.New(fmt.Sprintf("Generator.execTemplate.template.ParseFS: %v", err))
	}

	return tmpl.Execute(w, g.spec)
}

func (g *Generator) format(buf []byte) ([]byte, error) {
	src, err := format.Source(buf)
	if err != nil {
		return nil, fmt.Errorf("Generator.format: %s\n", err)
	}
	return src, nil
}
