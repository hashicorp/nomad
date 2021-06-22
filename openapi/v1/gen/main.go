package main

import (
	"errors"
	"fmt"
	"github.com/hashicorp/nomad/api"
	"github.com/modern-go/reflect2"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"golang.org/x/tools/go/ast/inspector"
	"golang.org/x/tools/go/packages"
	"log"
	"os"
	"reflect"
	"strings"
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
	const loadMode = packages.NeedName |
		packages.NeedFiles |
		packages.NeedCompiledGoFiles |
		packages.NeedImports |
		packages.NeedDeps |
		packages.NeedExportsFile |
		packages.NeedTypes |
		packages.NeedSyntax |
		packages.NeedTypesInfo |
		packages.NeedTypesSizes |
		packages.NeedModule

	cfg := &packages.Config{Mode: loadMode}

	pkgs, err := packages.Load(cfg, "../../../api/")
	if err != nil {
		return err
	}
	if packages.PrintErrors(pkgs) > 0 {
		return errors.New("error loading packages - see stdout for details")
	}

	var inputBuilder strings.Builder

	for _, pkg := range pkgs {
		for _, goFile := range pkg.GoFiles {
			// Create the AST by parsing src.
			fset := token.NewFileSet() // positions are relative to fset
			f, err := parser.ParseFile(fset, goFile, nil, 0)
			if err != nil {
				panic(err)
			}

			for _, node := range f.Decls {
				switch node.(type) {

				case *ast.GenDecl:
					genDecl := node.(*ast.GenDecl)
					for _, spec := range genDecl.Specs {
						switch spec.(type) {
						case *ast.TypeSpec:
							typeSpec := spec.(*ast.TypeSpec)

							switch typeSpec.Type.(type) {
							case *ast.StructType:
								if hasClient(typeSpec) {
									continue
								} else {
									// fmt.Println(fmt.Sprintf("%+v", typeSpec))
									inputBuilder.WriteString(fmt.Sprintf("\t&api.%s,\n", typeSpec.Name.Name))
								}
							}
						}
					}
				}
			}
		}
	}

	f, err := os.OpenFile("input.go", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		log.Fatal(err)
	}

	_, err = f.WriteString(strings.Replace(inputTmpl, "{{REPLACE}}", inputBuilder.String(), 1))
	if err != nil {
		log.Fatal(err)
	}

	if err := f.Close(); err != nil {
		log.Fatal(err)
	}

	return nil
}

func hasClient(typeSpec *ast.TypeSpec) bool {
	structType := typeSpec.Type.(*ast.StructType)
	for _, field := range structType.Fields.List {
		for _, name := range field.Names {
			if name.Name == "client" {
				return true
			}
		}
	}
	return false
}

func createInstance(name string) interface{} {
	test := api.ACLPolicies{}
	fmt.Println(fmt.Sprintf("force package include %+v", test))
	t := reflect2.TypeByPackageName("github.com/hashicorp/nomad/api", name)
	return reflect.New(t.Type1()).Elem().Interface()
}

var inputTmpl = `
package main

import (
	"github.com/hashicorp/nomad/api"
)

inputs := [
{{REPLACE}}
]
`

