package main

import (
	"bytes"
	"embed"
	"fmt"
	"go/format"
	"io"
	"os"
)

//go:embed spec.tmpl
var specTmpl embed.FS

//go:embed param.tmpl
var paramTmpl embed.FS

//go:embed header.tmpl
var headerTmpl embed.FS

//go:embed response.tmpl
var responseTmpl

//go:embed schema.tmpl
var schemaTmpl embed.FS

//go:embed paths.tmpl
var pathsTmpl embed.FS

// Generator iterates over the input source and configuration, and aggregates a
// data model a that can be used to render an openapi from the template.
type Generator struct {
	Builder SpecBuilder
}

// RenderTemplate outputs an OpenAPI spec from the Spec model and template file.
func (s *Spec) RenderTemplate() error {
	if err := s.Model.Validate(s.Context); err != nil {
		return fmt.Errorf("Spec.RenderTemplate.Model.Validate: %v\n", err)
	}

	var formatted []byte
	var err error
	var buf bytes.Buffer
	if err = s.execTemplate(&buf, specTmpl); err != nil {
		return err
	}

	if formatted, err = s.format(buf.Bytes()); err != nil {
		return err
	}

	if err = os.WriteFile(s.OutputFile, formatted, 0644); err != nil {
		return fmt.Errorf("Spec.os.WriteFile: %s\n", err)
	}

	return nil
}

func (s *Spec) execTemplate(w io.Writer, file embed.FS) error {
	tmpl, err := template.ParseFS(file, "*")
	if err != nil {
		return errors.New(fmt.Sprintf("Spec.execTemplate.template.ParseFS: %v", err))
	}

	return tmpl.Execute(w, s)
}

func (s *Spec) format(buf []byte) ([]byte, error) {
	src, err := format.Source(buf)
	if err != nil {
		return nil, fmt.Errorf("Spec.format: %s\n", err)
	}
	return src, nil
}
