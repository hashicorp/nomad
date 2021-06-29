package main

import (
	"bytes"
	"context"
	"embed"
	"errors"
	"fmt"
	openapi3 "github.com/getkin/kin-openapi/openapi3"
	"go/format"
	"io"
	"os"
	"text/template"
)

// SpecBuilder allows specifying different static analysis behaviors to that the
// framework can target any extant API.
type SpecBuilder interface {
	Build() (Spec, error)
}

type Spec struct {
	Context     context.Context
	OutputFile  string // path to write file to
	SpecVersion string
	Model       openapi3.T
	//Info        Info
	//Servers     []Server
	//Components  Components
	//Paths       []Path
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

//func (s *Spec) isValid() bool {
//	// TODO: Add validation logic here
//	return false
//}
//
//type Info struct {
//	Title          string
//	Description    string
//	TermsOfService string
//	Contact        Contact
//	License        License
//	Version        string
//}
//
//type Contact struct {
//	Name  string
//	URL   string
//	Email string
//}
//
//type License struct {
//	Name string
//	URL  string
//}
//
//type Server struct {
//	URL         string
//	Description string
//	Variables   []ServerVariable
//}
//
//type ServerVariable struct {
//	Name        string
//	Enum        []string
//	Default     string
//	Description string
//}
//
//type Components struct {
//	Schemas         []Schema
//	Responses       []Response
//	Parameters      []Parameter
//	Examples        []Example
//	RequestBodies   []RequestBody
//	Headers         []Header
//	SecuritySchemes SecurityScheme
//	Links           []Link
//	Callbacks       []Callback
//}
//
//type SecurityScheme struct {
//}
//
//type ParameterType string
//
//const (
//	InHeader ParameterType = "header"
//	InQuery  ParameterType = "query"
//	InPath   ParameterType = "Path"
//)
//
//type Parameter struct {
//	Type ParameterType
//}
//
//type Header struct {
//}
//
//type Response struct {
//}
//
//type Schema struct {
//	Name string
//}
//
//func (s *Schema) String() {
//	return fmt.Sprintf("")
//}
//
//type Example struct {
//}
//
//type Link struct {
//}
//
//type Callback struct {
//}
//
//type RequestBody struct {
//}
//
//type Path struct {
//}
