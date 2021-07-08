package main

import (
	"fmt"
	"github.com/getkin/kin-openapi/openapi3"
	"go/ast"
	"go/token"
	"go/types"
	"golang.org/x/tools/go/cfg"
	"golang.org/x/tools/go/packages"
	"net/http"
	"strings"
)

func NewVarAdapter(spec *ast.ValueSpec, analyzer *Analyzer) *VarAdapter {
	return &VarAdapter{
		spec,
		analyzer,
	}
}

type VarAdapter struct {
	spec     *ast.ValueSpec
	analyzer *Analyzer
}

func (v *VarAdapter) Name() string {
	return v.spec.Names[0].Name
}

func (v *VarAdapter) TypeName() string {
	return v.analyzer.FormatTypeName(v.expr().X.(*ast.Ident).Name, v.expr().Sel.Name)
}

func (v *VarAdapter) expr() *ast.SelectorExpr {
	return v.spec.Type.(*ast.SelectorExpr)
}

func (v *VarAdapter) Type() types.Object {
	return v.analyzer.GetTypeByName(v.TypeName())
}

type PathItemAdapter struct {
	method  string // GET, PUT etc.
	Handler *HandlerFuncAdapter
}

// GetMethod returns a string that maps to the net/http method this PathItemAdapter
// represents e.g. GET, POST, PUT
func (pia *PathItemAdapter) GetMethod() string {
	method := "unknown"

	return method
}

// GetInputParameterRefs creates an ParameterRef slice by inspecting the source code
func (pia *PathItemAdapter) GetInputParameterRefs() []*openapi3.ParameterRef {
	var refs []*openapi3.ParameterRef

	//for _, param := range t.Type.Params.List {
	//	params = fmt.Sprintf("%s|%s ", param.Names[0].Name, param.Type)
	//}

	return refs
}

// GetRequestBodyRef creates a RequestBodyRef by inspecting the source code
func (pia *PathItemAdapter) GetRequestBodyRef() *openapi3.RequestBodyRef {
	ref := &openapi3.RequestBodyRef{}

	return ref
}

// GetResponseSchemaRef creates a SchemaRef by inspecting the source code. This
// is intended as a debug function. Use GetResponseRefs to generate a spec.
func (pia *PathItemAdapter) GetResponseSchemaRef() *openapi3.SchemaRef {
	ref := &openapi3.SchemaRef{}

	return ref
}

// GetResponseRefs creates a slice of ResponseRefs by inspecting the source code
func (pia *PathItemAdapter) GetResponseRefs() []*openapi3.ResponseRef {
	var refs []*openapi3.ResponseRef

	return refs
}

type HandlerFuncAdapter struct {
	Package  *packages.Package
	Func     *types.Func
	FuncDecl *ast.FuncDecl

	logger   loggerFunc
	analyzer *Analyzer
	fileSet  *token.FileSet

	Variables map[string]*VarAdapter
	// The CFG does contain Return statements; even implicit returns are materialized
	// (at the position of the function's closing brace).
	// CFG does not record conditions associated with conditional branch edges,
	//nor the short-circuit semantics of the && and || operators, nor abnormal
	//control flow caused by panic. If you need this information, use golang.org/x/tools/go/ssa instead.
	Cfg *cfg.CFG
}

func (h *HandlerFuncAdapter) GetPath() string {
	// TODO: Resolve the path
	return h.Name()
}

// TODO: Find a way to make this injectable
var supportedMethods = []string{http.MethodGet, http.MethodDelete, http.MethodPut, http.MethodPost}

func (h *HandlerFuncAdapter) newPathItemAdapter(method string) (*PathItemAdapter, error) {
	isSupportedMethod := false
	for _, supportedMethod := range supportedMethods {
		if supportedMethod == method {
			isSupportedMethod = true
		}
	}
	if !isSupportedMethod {
		return nil, fmt.Errorf("HandlerFuncAdapter.newPathItemAdapter: method %s not supported", method)
	}

	return &PathItemAdapter{method: method}, nil
}

func (h *HandlerFuncAdapter) Name() string {
	return h.Func.Name()
}

func (h *HandlerFuncAdapter) GetSource() (string, error) {
	return h.analyzer.GetSource(h.FuncDecl.Body, h.fileSet)
}

func (h *HandlerFuncAdapter) debugReturnSource(idx int) string {
	result, _ := h.GetResultByIndex(idx)
	src, _ := h.analyzer.GetSource(result, h.fileSet)
	src = strings.Replace(src, "\n", "", -1)
	src = strings.Replace(src, "\t", "", -1)
	return src
}

// IsIntermediateFunc is used to determine if an HTTP Handler is actually
// an intermediate function that in turn calls other functions to handle
// different HTTP methods, path parameters, etc.
func (h *HandlerFuncAdapter) IsIntermediateFunc() bool {
	// TODO: Find a way to detect that this is a FooSpecificRequestFunc
	return false
}

// visitHandlerFunc makes several passes over the Handler FuncDecl. Order matters
// and trying to this all in one pass is likely not an option.
func (h *HandlerFuncAdapter) visitHandlerFunc() error {
	if err := h.FindVariables(); err != nil {
		return err
	}
	if _, err := h.GetReturnSchema(); err != nil {
		return err
	}

	return nil
}

func (h *HandlerFuncAdapter) FindVariables() error {
	if h.Variables == nil {
		h.Variables = make(map[string]*VarAdapter)
	}
	var variableVisitor = func(node ast.Node) bool {
		switch node.(type) {
		case *ast.GenDecl:
			if node.(*ast.GenDecl).Tok != token.VAR {
				return true
			}

			for i, spec := range node.(*ast.GenDecl).Specs {
				switch spec.(type) {
				case *ast.ValueSpec:
					varAdapter := NewVarAdapter(spec.(*ast.ValueSpec), h.analyzer)
					// @@@@ DEBUG
					if varAdapter.Type() == nil {
						panic(fmt.Sprintf("ValueSpecType: %v", node.(*ast.GenDecl).Specs[i].(*ast.ValueSpec).Type))
					}

					if _, ok := h.Variables[varAdapter.Name()]; !ok {
						h.Variables[varAdapter.Name()] = varAdapter
					}
				}
			}
		}
		return true
	}

	ast.Inspect(h.FuncDecl, variableVisitor)

	if h.analyzer.debugOptions.printVariables {
		for k, v := range h.Variables {
			h.analyzer.Logger(h.Func.Name(), "variable", k, "has TypeName:", v.TypeName(), "with type signature", v.Type().String())
		}
	}

	return nil
}

func (h *HandlerFuncAdapter) GetReturnSchema() (*openapi3.SchemaRef, error) {
	result, err := h.GetResultByIndex(0)
	if err != nil {
		return nil, err
	}

	var outType types.Object

	outVisitor := func(node ast.Node) bool {
		switch t := node.(type) {
		case *ast.Ident:
			var v *VarAdapter
			var ok bool
			// @@@@ Debug should never happen if all loading is done correctly.
			if v, ok = h.Variables[t.Name]; !ok {
				panic("HandlerFuncAdapter.GetReturnSchema failed to find variable " + t.Name)
			}
			outType = v.Type()
		case *ast.SelectorExpr:
			switch xt := t.X.(type) {
			case *ast.Ident:
				var v *VarAdapter
				var ok bool
				varName := h.analyzer.FormatTypeName(xt.Name, t.Sel.Name)
				// @@@@ Debug should never happen if all loading is done correctly.

				// Left off here - I've got to solve the indirection problem e.g. out.Jobs

				if v, ok = h.Variables[varName]; !ok {
					panic("HandlerFuncAdapter.GetReturnSchema failed to find variable " + xt.Name)
				}
				outType = v.Type()
			default:
				panic(fmt.Sprintf("HandlerFuncAdapter.GetReturnSchema: unhandled type %v", xt))
			}
		default:
			panic(fmt.Sprintf("HandlerFuncAdapter.GetReturnSchema: unhandled type %v", t))
		}
		return true
	}

	ast.Inspect(result, outVisitor)

	return h.toSchemaRef(outType), nil
}

func (h *HandlerFuncAdapter) GetReturnStmts() []*ast.ReturnStmt {
	var returnStmts []*ast.ReturnStmt

	returnVisitor := func(node ast.Node) bool {
		switch t := node.(type) {
		case *ast.ReturnStmt:
			returnStmts = append(returnStmts, t)
		}
		return true
	}

	ast.Inspect(h.FuncDecl, returnVisitor)

	return returnStmts
}

func (h *HandlerFuncAdapter) GetResultByIndex(idx int) (ast.Expr, error) {
	returnStmts := h.GetReturnStmts()
	if len(returnStmts) < 1 {
		return nil, fmt.Errorf("HandlerFuncAdapter.GetResultByIndex: no return statement found")
	}

	finalReturn := returnStmts[len(returnStmts)-1]
	if finalReturn == nil {
		return nil, fmt.Errorf("HandlerFuncAdapter.GetResultByIndex: finalReturn does not exist")
	}

	if len(finalReturn.Results) < idx+1 {
		return nil, fmt.Errorf("HandlerFuncAdapter.GetResultByIndex: invalid index")
	}
	return finalReturn.Results[idx].(ast.Expr), nil
}

func (h *HandlerFuncAdapter) toSchemaRef(schemaType types.Object) *openapi3.SchemaRef {
	if schemaType == nil {
		return nil
	}

	schemaRef := &openapi3.SchemaRef{
		Ref: fmt.Sprintf("#/components/schemas/%s", schemaType.Name()),
		Value: &openapi3.Schema{
			Type: schemaType.Name(),
		},
	}

	// TODO: need to read up on how to actually build one!

	return schemaRef
}
