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

func main() {
	var outDir string

	flag.StringVar(&outDir, "outDir", "./", "The output dir to save spec to")
	flag.Parse()

	g := &Generator{
		Builder: NomadSpecBuilder{},
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
