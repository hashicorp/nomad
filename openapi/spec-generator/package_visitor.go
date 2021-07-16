package main

import (
	"github.com/getkin/kin-openapi/openapi3"
	"go/ast"
	"go/token"
	"golang.org/x/tools/go/packages"
)

type PackageVisitor interface {
	Parse() error
	Analyzer() *Analyzer
	VisitFile(ast.Node) bool
	VisitPackages() error
	SetActiveFileSet(*token.FileSet)
	GetActiveFileSet() *token.FileSet
	SchemaRefs() map[string]*openapi3.SchemaRef
	ParameterRefs() map[string]*openapi3.ParameterRef
	HeaderRefs() map[string]*openapi3.HeaderRef
	RequestBodyRefs() map[string]*openapi3.RequestBodyRef
	CallbackRefs() map[string]*openapi3.CallbackRef
	ResponseRefs() map[string]*openapi3.ResponseRef
	HandlerAdapters() map[string]*handlerFuncAdapter
	DebugPrint()
}

type PackageConfig struct {
	Config   packages.Config
	Pattern  string
	Alias    string
	FileName string
}
