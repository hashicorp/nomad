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

type PathItemAdapter struct {
	method           string // GET, PUT etc.
	Package          *packages.Package
	Source           string
	Func             *types.Func
	FuncDecl         *ast.FuncDecl
	SchemaName       string
	SchemaTypeSpec   *ast.TypeSpec
	structs          map[string]*ast.TypeSpec
	logger           loggerFunc
	analyzer         *Analyzer
	fileSet          *token.FileSet
	returnStatements []*ast.ReturnStmt
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
	Path     string
	Package  *packages.Package
	Func     *types.Func
	FuncDecl *ast.FuncDecl

	// The CFG does contain Return statements; even implicit returns are materialized
	// (at the position of the function's closing brace).

	// CFG does not record conditions associated with conditional branch edges,
	//nor the short-circuit semantics of the && and || operators, nor abnormal
	//control flow caused by panic. If you need this information, use golang.org/x/tools/go/ssa instead.
	Cfg     *cfg.CFG
	Structs map[string]*ast.TypeSpec

	logger           loggerFunc
	analyzer         *Analyzer
	fileSet          *token.FileSet
	returnStatements []*ast.ReturnStmt

	ResponseType *ast.TypeSpec
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

func (h *HandlerFuncAdapter) IsHelperFunction() bool {
	return h.ResponseType == nil
}

func (f *HandlerFuncAdapter) visitFunc(node ast.Node) bool {
	switch t := node.(type) {
	case *ast.ReturnStmt:
		f.returnStatements = append(f.returnStatements, t)
		// TODO: This is where I'll have to come back and handle nutty things like JobSpecificRequest
		//case *ast.BlockStmt:
		//	for _, stmt := range t.List {
		//		f.logger(f.analyzer.GetSource(stmt, f.fileSet))
		//	}
		//case *ast.BranchStmt:
		//	f.logger(f.analyzer.GetSource(t, f.fileSet))
		//
	}
	return true
}

func (f *HandlerFuncAdapter) processVisitResults() error {
	if _, err := f.GetReturnSchema(); err != nil {
		return err
	}
	return nil
}

func (f *HandlerFuncAdapter) GetReturnSchema() (*openapi3.SchemaRef, error) {
	returnType := f.analyzer.GetReturnTypeByHandlerName(f.Name())

	f.logger(returnType)
	result, err := f.GetResultByIndex(0)
	if err != nil {
		return nil, err
	}

	var outTypeName string

	outVisitor := func(node ast.Node) bool {
		switch t := node.(type) {
		case *ast.SelectorExpr:
			switch xt := t.X.(type) {
			case *ast.Ident:
				foo := f.analyzer.Defs(xt)
				f.logger(fmt.Sprintf("%v", foo))
				if xt.Name == "out" {
					outTypeName = xt.Obj.Decl.(*ast.ValueSpec).Type.(*ast.SelectorExpr).Sel.Name
				} else {
					f.logger("Ident name: " + xt.Name)
				}
			}
		default:
			returnSrc := f.debugReturnSource(0)
			if returnSrc != "nil" {
				variable := f.analyzer.GetFuncVariable(returnSrc, f.FuncDecl)
				variableSrc, _ := f.analyzer.GetSource(variable, f.fileSet)
				f.logger(fmt.Sprintf("returnSrc: %s - variableSrc: %s", returnSrc, variableSrc))
			} else {
				f.logger(fmt.Sprintf("%s: out var name: %s type: %v", f.Name(), returnSrc, t))
			}
		}
		return true
	}

	ast.Inspect(result, outVisitor)

	if len(outTypeName) > 0 {
		// DEBUG
		//if outTypeName == "JobSummaryResponse" {
		//	for k, _ := range f.Structs {
		//		if strings.Index(k, "Job") > -1 {
		//			f.logger("Found Key: %s", k)
		//		}
		//	}
		//}
		f.logger(fmt.Sprintf("Func: %s outTypeName: %s", f.Name(), outTypeName))
		var ok bool
		if f.ResponseType, ok = f.Structs["api."+outTypeName]; ok {
			return &openapi3.SchemaRef{}, nil
		}
	}

	return &openapi3.SchemaRef{}, nil
}

func (f *HandlerFuncAdapter) GetResultByIndex(idx int) (ast.Expr, error) {
	//for _, block := range f.Cfg.Blocks {
	//	//TODO: Left off here
	//}

	if len(f.returnStatements) < 1 {
		return nil, fmt.Errorf("HandlerFuncAdapter.GetResultByIndex: no return statement found")
	}

	finalReturn := f.returnStatements[len(f.returnStatements)-1]
	if finalReturn == nil {
		return nil, fmt.Errorf("HandlerFuncAdapter.GetResultByIndex: finalReturn does not exist")
	}

	if len(finalReturn.Results) < idx+1 {
		return nil, fmt.Errorf("HandlerFuncAdapter.GetResultByIndex: invalid index")
	}
	return finalReturn.Results[idx].(ast.Expr), nil
}
