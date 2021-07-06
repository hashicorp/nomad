package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"golang.org/x/tools/go/packages"
)

type PackageVisitor interface {
	VisitNode(node ast.Node) bool
	VisitPackages([]*packages.Package) error
	GetActiveFileSet() *token.FileSet
	DebugPrint()
}

type PackageConfig struct {
	Config  packages.Config
	Pattern string
}

// PackageParser encapsulates the necessary configuration and logic to load and parse
// a source code package. Config.Dir can be set to an absolute or relative path
// containing the source code. Pattern is used by the loader to determine which
// elements of the package to load. To load everything, past ".". See full docs at
// https://pkg.go.dev/golang.org/x/tools/go/packages#section-documentation
type PackageParser struct {
	Packages      []*PackageConfig
	Visitor       PackageVisitor
	activeFileSet *token.FileSet
}

func (p *PackageParser) Parse() error {
	var err error
	var pkgs []*packages.Package

	for _, pkgConfig := range p.Packages {
		var ps []*packages.Package
		if ps, err = packages.Load(&pkgConfig.Config, pkgConfig.Pattern); err != nil {
			return fmt.Errorf("PackageParser.Parse.packages.Load: %v", err)
		}

		pkgs = append(pkgs, ps...)
	}

	if err = p.Visitor.VisitPackages(pkgs); err != nil {
		return err
	}

	p.Visitor.DebugPrint()

	return nil
}
