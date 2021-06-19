package main

import (
	"errors"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"

	_ "github.com/hashicorp/nomad/api"
	"golang.org/x/tools/go/ast/inspector"
	"golang.org/x/tools/go/packages"
)

func main() {
	if err := genSchema(); err != nil {
		fmt.Println(fmt.Sprintf("openapi.v1.gen failed with error: %+v", err))
	}
}

func genSchema() error {
	pkg, err := build.Default.Import("github.com/hashicorp/nomad/api", "../../../", 0)
	if err != nil {
		return err
	}

	for _, fileName := range pkg.GoFiles {
		fset := token.NewFileSet()
		filePath := "../../../api/" + fileName
		f, _ := parser.ParseFile(fset, filePath, nil, 0)

		inspect := inspector.New([]*ast.File{f})

		var got []string
		fn := func(n ast.Node, push bool) bool {
			if push {
				ast.Print(fset, f)
				got = append(got, buildSchema(n))
			}
			return true
		}

		nodeTypes := []ast.Node{
			(*ast.StructType)(nil),
		}
		got = nil
		inspect.Nodes(nodeTypes, fn)

		fmt.Println(fmt.Sprintf("got: %+v", got))
	}

	return nil
}

func buildSchema(n ast.Node) string {
	return fmt.Sprintf("%+v", n.(*ast.StructType).Fields)
}

func loadPackages() error {
	cfg := &packages.Config{Mode: packages.NeedFiles | packages.NeedSyntax}
	pkgs, err := packages.Load(cfg, "../../../api/")
	if err != nil {
		return err
	}
	if packages.PrintErrors(pkgs) > 0 {
		return errors.New("error loading packages - see stdout for details")
	}

	for _, pkg := range pkgs {
		fmt.Println(pkg.ID, pkg.GoFiles)
	}

	return nil
}
