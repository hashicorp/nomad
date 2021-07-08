package main

import (
	"go/ast"
	"go/token"
	"golang.org/x/tools/go/packages"
)

type PackageVisitor interface {
	Analyzer() *Analyzer
	VisitFile(ast.Node) bool
	VisitPackages() error
	SetActiveFileSet(*token.FileSet)
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
	Analyzer      *Analyzer
	Visitor       PackageVisitor
	activeFileSet *token.FileSet
}

func (p *PackageParser) Parse() error {
	var err error

	if err = p.Visitor.VisitPackages(); err != nil {
		return err
	}

	p.Visitor.DebugPrint()

	return nil
}
