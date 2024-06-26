package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"

	"github.com/hashicorp/go-hclog"
	"golang.org/x/tools/go/packages"
)

// ParsedPackage represents the completed first stage of parsing a set of Go
// source files.
type ParsedPackage struct {
	Name string
	Dir  string

	// Files represents the set of individual Go source files
	Files []*ast.File

	// FileSet is an efficient representation of all the individual source lines
	// and token positions, which the downstream go/ast package functions will
	// need
	FileSet *token.FileSet
}

func loadPackages(l hclog.Logger, packageDir string) ([]*ParsedPackage, error) {
	pkgs, err := packages.Load(&packages.Config{Dir: packageDir}, ".")
	if err != nil {
		return nil, err
	}

	parsedPkgs := []*ParsedPackage{}
	for _, pkg := range pkgs {
		parsedPkg, err := parsePackage(l, pkg)
		if err != nil {
			return nil, fmt.Errorf("failed to parse package %q: %w", pkg.Name, err)
		}
		parsedPkg.Dir = packageDir
		parsedPkgs = append(parsedPkgs, parsedPkg)
	}

	return parsedPkgs, nil
}

// parsePackage processes all the Go source files in the package
func parsePackage(l hclog.Logger, pkg *packages.Package) (*ParsedPackage, error) {

	if len(pkg.Errors) > 0 {
		return nil, pkg.Errors[0]
	}
	l.Debug("parsed package", "package", pkg.Name)

	fset := token.NewFileSet()
	files := []*ast.File{}

	for _, goFile := range pkg.GoFiles {
		if strings.Contains(goFile, ".generated.go") {
			continue // skip any generated files
		}
		// the FileSet gets mutated with each file parsed
		file, err := parser.ParseFile(fset, goFile, nil, parser.ParseComments)
		if err != nil {
			return nil, fmt.Errorf("could not parse file: %v\n", err)
		}
		files = append(files, file)
	}

	return &ParsedPackage{
		Name:    pkg.Name,
		FileSet: fset,
		Files:   files,
	}, nil
}
