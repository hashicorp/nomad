package main

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"io"
	"os"
	"strings"
	"text/template"

	"github.com/hashicorp/go-hclog"
)

//go:embed structs.copy.tmpl
var copyTmpl embed.FS

//go:embed structs.equals.tmpl
var equalsTmpl embed.FS

//go:embed structs.diff.tmpl
var diffTmpl embed.FS

//go:embed structs.merge.tmpl
var mergeTmpl embed.FS

// Generator holds the state of the analysis. Used as the data model and executor
// to render templates.
type Generator struct {
	logger     hclog.Logger
	packageDir string

	files   []*ast.File
	fileset *token.FileSet
}

func NewGenerator(packageDir string, logger hclog.Logger) *Generator {
	return &Generator{
		packageDir: packageDir,
		logger:     logger,
		files:      []*ast.File{},
		fileset:    &token.FileSet{},
	}
}

func (g *Generator) generate(results []*Result) error {
	var err error
	for _, result := range results {
		if err = g.render(result); err != nil {
			return errors.New(fmt.Sprintf("generate.%s: %v", result.Method, err))
		}
	}
	return nil
}

func (g *Generator) render(result *Result) error {
	var err error
	targetFileName := fmt.Sprintf("./%s.%s.generated.go",
		result.Package, strings.ToLower(result.Method))

	var templateFile embed.FS

	switch result.Method {
	case "Copy":
		templateFile = copyTmpl
	case "Equals":
		templateFile = equalsTmpl
	case "Diff":
		templateFile = diffTmpl
	case "Merge":
		templateFile = mergeTmpl
	}

	var buf bytes.Buffer
	err = g.execTemplate(&buf, templateFile, result)
	if err != nil {
		return err
	}

	formatted := g.format(buf.Bytes())

	err = os.WriteFile(targetFileName, formatted, 0644)
	if err != nil {
		return err
	}

	return nil
}

func (g *Generator) execTemplate(w io.Writer, file embed.FS, result *Result) error {
	if len(result.Targets) < 1 {
		return errors.New("generate.render.execTemplate: no targets found")
	}
	tmpl, err := template.ParseFS(file, "*")
	if err != nil {
		return errors.New(fmt.Sprintf("generate.render.execTemplate: %v", err))
	}
	return tmpl.Execute(w, result)
}

func (g *Generator) format(buf []byte) []byte {
	src, err := format.Source(buf)
	if err != nil {
		fmt.Printf("invalid Go generated: %s\n", err) // should never happen
		return buf
	}
	return src
}
