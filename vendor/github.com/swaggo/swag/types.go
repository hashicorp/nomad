package swag

import (
	"github.com/go-openapi/spec"
	"go/ast"
)

//Schema parsed schema
type Schema struct {
	PkgPath      string //package import path used to rename Name of a definition int case of conflict
	Name         string //Name in definitions
	*spec.Schema        //
}

//TypeSpecDef the whole information of a typeSpec
type TypeSpecDef struct {
	//path of package starting from under ${GOPATH}/src or from module path in go.mod
	PkgPath string

	//ast file where TypeSpec is
	File *ast.File

	//the TypeSpec of this type definition
	TypeSpec *ast.TypeSpec
}

//Name name of the typeSpec
func (t *TypeSpecDef) Name() string {
	return t.TypeSpec.Name.Name
}

//FullName full name of the typeSpec
func (t *TypeSpecDef) FullName() string {
	return fullTypeName(t.File.Name.Name, t.TypeSpec.Name.Name)
}

//AstFileInfo information of a ast.File
type AstFileInfo struct {
	//File ast.File
	File *ast.File

	//Path path of the ast.File
	Path string

	//PackagePath package import path of the ast.File
	PackagePath string
}

//PackageDefinitions files and definition in a package
type PackageDefinitions struct {
	//package name
	Name string

	//files in this package, map key is file's relative path starting package path
	Files map[string]*ast.File

	//definitions in this package, map key is typeName
	TypeDefinitions map[string]*TypeSpecDef
}
