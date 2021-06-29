package main

import (
	"bytes"
	"embed"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"os"
	"strings"
	"text/template"

	"golang.org/x/tools/go/packages"
)

//go:embed spec.tmpl
var specTmpl embed.FS

//go:embed components.tmpl
var componentsTmpl embed.FS

//go:embed param.tmpl
var paramTmpl embed.FS

//go:embed header.tmpl
var headerTmpl embed.FS

//go:embed response.tmpl
var response.tmpl

//go:embed schema.tmpl
var schemaTmpl embed.FS

//go:embed path.tmpl
var pathTmpl embed.FS

//go:embed get.tmpl
var getTmpl embed.FS

//go:embed post.tmpl
var postTmpl embed.FS

//go:embed delete.tmpl
var deleteTmpl embed.FS

type stringSliceFlag []string

func (s *stringSliceFlag) String() string {
	return fmt.Sprintf("%s", *s)
}

func (s *stringSliceFlag) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func main() {

	var excludedFieldFlags stringSliceFlag
	var typeNameFlags stringSliceFlag
	var methodFlags stringSliceFlag
	var packageDir string

	flag.Var(&excludedFieldFlags, "exclude", "list of Fields to exclude from Copy")
	flag.Var(&typeNameFlags, "type", "types for which to generate Copy methodFlags")
	flag.Var(&methodFlags, "method", "methodFlags to generate - defaults to all")
	flag.StringVar(&packageDir, "packageDir", "./", "The source dir to target")
	flag.Parse()

	if len(typeNameFlags) == 0 {
		fmt.Println("at least one -type flag needed to generate Copy")
		os.Exit(2)
	}

	g := &Generator{
		packageDir:     packageDir,
		typeNames:      typeNameFlags,
		methods:        methodFlags,
		excludedFields: excludedFieldFlags,
		typeSpecs:      map[string]*TypeSpecNode{},
	}
	err := run(g)
	if err != nil {
		fmt.Println(err)
		os.Exit(2)
	}
}

func run(g *Generator) error {

	var err error
	var pkgs []*packages.Package
	if pkgs, err = g.loadPackages(); err != nil {
		return fmt.Errorf("error loading packages: %v", err)
	}

	if err = g.parsePackages(pkgs); err != nil {
		return fmt.Errorf("error parsing packages: %v", err)
	}
	if len(pkgs) == 0 {
		return fmt.Errorf("did not parse any packages")
	}

	if err = g.analyze(); err != nil {
		return fmt.Errorf("error analyzing: %v", err)
	}
	if len(g.typeSpecs) == 0 {
		return fmt.Errorf("did not analyze any types")
	}

	if err = g.generate(); err != nil {
		return fmt.Errorf("error generating: %v", err)
	}
	return nil
}
