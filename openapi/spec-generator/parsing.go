package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"golang.org/x/tools/go/packages"
)

type NodeVisitor interface {
	VisitNode(node ast.Node) bool
	GetActiveFileSet() *token.FileSet // LIFO accessor for FilesSets
	SetActiveFileSet(*token.FileSet)  // LIFO accesor for Files
	Files() []*ast.File
	FileSets() []*token.FileSet
	DebugPrint()
}

// ParseResult encapsulate the output of a parse operation.
type ParseResult struct {
	Package  *packages.Package
	Files    []*ast.File
	FileSets []*token.FileSet
}

// PackageParser encapsulates the necessary configuration and logic to load and parse
// a source code package. Config.Dir can be set to an absolute or relative path
// containing the source code. Pattern is used by the loader to determine which
// elements of the package to load. To load everything, past ".". See full docs at
// https://pkg.go.dev/golang.org/x/tools/go/packages#section-documentation
type PackageParser struct {
	Config         packages.Config
	Pattern        string
	Debug          bool
	Visitor        NodeVisitor
	CurrentFile    *ast.File
	CurrentFileSet *token.FileSet
}

func (p *PackageParser) Parse() (*[]*ParseResult, error) {
	var err error
	var pkgs []*packages.Package

	if pkgs, err = packages.Load(&p.Config, p.Pattern); err != nil {
		return nil, fmt.Errorf("PackageParser.Parse.packages.Load: %v", err)
	}

	var results *[]*ParseResult
	if results, err = p.parsePackages(pkgs); err != nil {
		return nil, err
	}

	return results, nil
}

// parsePackages iterates over the package source and ensures each go file is processed.
func (p *PackageParser) parsePackages(pkgs []*packages.Package) (*[]*ParseResult, error) {
	var results []*ParseResult
	for _, pkg := range pkgs {

		if len(pkg.Errors) > 0 {
			return nil, fmt.Errorf("PackageParser.parsePackages.pkg.Errors: %v\n", pkg.Errors[0])
		}

		result := &ParseResult{
			Package: pkg,
		}

		for _, goFile := range pkg.GoFiles {
			if err := p.parseGoFile(goFile, result); err != nil {
				return nil, err
			}
		}

		results = append(results, result)
	}

	return &results, nil
}

// parseGoFile parses an individual go file and adds both the file and the fileSet
// to the parse result.
func (p *PackageParser) parseGoFile(goFile string, result *ParseResult) error {
	// Create the AST by parsing src.
	fileSet := token.NewFileSet() // positions are relative to fset
	file, err := parser.ParseFile(fileSet, goFile, nil, 0)
	if err != nil {
		return fmt.Errorf("PackageParser.parseGoFile: %v\n", err)
	}

	result.Files = append(result.Files, file)
	result.FileSets = append(result.FileSets, fileSet)

	p.CurrentFile = file
	p.CurrentFileSet = fileSet

	p.Visitor.SetActiveFileSet(fileSet)
	ast.Inspect(file, p.Visitor.VisitNode)

	p.Visitor.DebugPrint()

	return nil
}
