package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"go/types"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/refactor/satisfy"
	"strings"
)

type ParamLocation string

const (
	inHeader ParamLocation = "header"
	inPath   ParamLocation = "path"
	inQuery  ParamLocation = "query"
)

type ParamInfo struct {
	Name        string
	Description string
	Type        string
	Location    ParamLocation
}

type HeaderInfo struct {
	Name        string
	Description string
	Type        string
}

type HandlerFuncInfo struct {
	Path             string
	Source           string
	Func             *types.Func
	FuncDecl         *ast.FuncDecl
	Structs          map[string]*ast.TypeSpec
	Params           []*ParamInfo
	ResponseExpr     *ast.Expr
	ResponseType     *ast.TypeSpec
	ResponseHeaders  []*HeaderInfo
	logger           loggerFunc
	analyzer         *Analyzer
	fileSet          *token.FileSet
	returnStatements []*ast.ReturnStmt
}

func (h *HandlerFuncInfo) Name() string {
	return h.Func.Name()
}

func (f *HandlerFuncInfo) visitFunc(node ast.Node) bool {
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

func (f *HandlerFuncInfo) processVisitResults() error {
	if err := f.resolveResponseType(); err != nil {
		return err
	}
	return nil
}

func (f *HandlerFuncInfo) resolveResponseType() error {
	if len(f.returnStatements) < 1 {
		return fmt.Errorf("HandlerFuncInfo.resolveResponseType: no return statement found")
	}

	// If return statement returns nil  as first value then this is a
	// FooSpecificRequest Handlers. Happy Accident!!!
	// TODO: Vet this idea
	var responseExpr ast.Expr
	var responseType *ast.TypeSpec
	var err error
	if responseType, err = f.ResolveReturnType(); err != nil {
		return err
	}

	// TODO: prove this is still true
	if responseType == nil {
		if err = f.moveToPathSwitchHandler(); err != nil {
			return err
		}
		return nil
	}

	f.ResponseExpr = &responseExpr
	f.ResponseType = responseType

	return nil
}

func (f *HandlerFuncInfo) ResolveReturnType() (*ast.TypeSpec, error) {
	var returnType *ast.TypeSpec

	finalReturn := f.returnStatements[len(f.returnStatements)-1]
	if finalReturn == nil {
		return returnType, fmt.Errorf("HandlerFuncInfo.ResolveReturnType: finalReturn does not exist")
	}

	result := finalReturn.Results[0]
	returnParamName, err := f.analyzer.GetSource(result, f.fileSet)
	if err != nil {
		return returnType, err
	}

	// gets the out type from the source string. Heavily dependent on the convention
	// of the out parameter being named out.
	sourceParts := strings.Split(f.Source, "var out ")
	outTypeName := strings.Split(sourceParts[1], " ")[0]

	var ok bool

	// If it returns nil, it should be a Foo specific handler
	if returnParamName == "nil" {
		return nil, nil
	} else if strings.Index(returnParamName, "out.") > -1 { // handle return of out type field
		var outType *ast.TypeSpec
		if outType, ok = f.Structs[outTypeName]; !ok {
			return returnType, fmt.Errorf("HandlerFuncInfo.ResolveReturnType: cannot resolve type for name %s", outTypeName)
		}
		returnTypeFieldName := strings.Split(returnParamName, "out.")[1]
		for _, field := range outType.Type.(*ast.StructType).Fields.List {
			for _, ident := range field.Names {
				i := field.Type.(*ast.Ident)
				fieldType := i.Name
				if ident.Name == returnTypeFieldName {
					returnType = f.Structs[fieldType]
				}
			}
		}

	} else if returnParamName == "out" { // handle direct return of out type
		if returnType, ok = f.Structs[outTypeName]; !ok {
			return returnType, fmt.Errorf("HandlerFuncInfo.ResolveReturnType: cannot resolve type for name %s", outTypeName)
		}
	} else {
		// handle things not named out
		return returnType, fmt.Errorf("ResolveReturnTypeName.Unhandled %s", returnParamName)
	}

	return returnType, nil
}

func (f *HandlerFuncInfo) moveToPathSwitchHandler() error {
	for _, retStmt := range f.returnStatements {
		src, err := f.analyzer.GetSource(retStmt, f.fileSet)
		if err != nil {
			return fmt.Errorf("HandlerFuncInfo.moveToPathSwitchHandler: cannot render source")
		}
		f.logger(fmt.Sprintf("%s.moveToPathSwitchHandler.finalReturn.source: %s", f.Name(), src))
	}
	return nil
}

type HTTPProfile struct {
	IsResponseWriter bool // net/http.ResponseWriter
	IsRequest        bool // *net/http.Request
	IsHandler        bool // net/http.Handler
}

// Analyzer provides a number of static analysis helper functions.
type Analyzer struct {
	finder *satisfy.Finder
}

func (a *Analyzer) Finder() *satisfy.Finder {
	if a.finder == nil {
		a.finder = &satisfy.Finder{}
	}
	return a.finder
}

func (a *Analyzer) GetSource(elem interface{}, fileSet *token.FileSet) (string, error) {
	// Try the happy path first
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fileSet, elem); err != nil {
		return "", err
	} else {
		return buf.String(), nil
	}
}

func (a *Analyzer) IsHttpHandler(typeDefFunc *types.Func) bool {
	if funcSignature, ok := typeDefFunc.Type().(*types.Signature); ok {
		// typeDefFunc.
		profile := HTTPProfile{}

		a.setHTTPProfile(funcSignature.Params(), &profile)
		a.setHTTPProfile(funcSignature.Results(), &profile)

		return profile.IsHandler || (profile.IsResponseWriter && profile.IsRequest)
	}

	return false
}

func (a *Analyzer) setHTTPProfile(tup *types.Tuple, result *HTTPProfile) {
	if tup == nil {
		return
	}

	for i := 0; i < tup.Len(); i++ {
		tupleMember := tup.At(i)
		objectType := tupleMember.Type().String()
		switch objectType {
		case "net/http.ResponseWriter":
			result.IsResponseWriter = true
		case "*net/http.Request":
			result.IsRequest = true
		case "net/http.Handler":
			result.IsHandler = true
		default:
			// capture cases such as function that return or accept functions
			// ex. (func(net/http.Handler) net/http.Handler, error)
			if strings.Contains(objectType, "net/http.ResponseWriter") {
				result.IsResponseWriter = true
			}
			if strings.Contains(objectType, "*net/http.Request") {
				result.IsRequest = true
			}
			if strings.Contains(objectType, "net/http.Handler") {
				result.IsHandler = true
			}
		}
	}

	return
}

func (a *Analyzer) GetHttpHandlers(pkg *packages.Package) map[string]*types.Func {
	httpHandlers := make(map[string]*types.Func)
	for _, typeDef := range pkg.TypesInfo.Defs {
		if typeDef != nil {
			if typeDefFunc, ok := typeDef.(*types.Func); ok {
				if a.IsHttpHandler(typeDefFunc) {
					httpHandlers[typeDefFunc.Name()] = typeDefFunc
				}
			}
		}
	}

	return httpHandlers
}

func (a *Analyzer) GetStructs(pkg *packages.Package) (map[string]*ast.TypeSpec, error) {
	var structMap = make(map[string]*ast.TypeSpec)
	fmtString := pkg.Name + ".%s"

	visitFunc := func(node ast.Node) bool {
		switch node.(type) {
		case *ast.TypeSpec:
			typeSpec := node.(*ast.TypeSpec)
			switch typeSpec.Type.(type) {
			case *ast.StructType:
				structMap[fmt.Sprintf(fmtString, typeSpec.Name)] = typeSpec
			}
		}
		return true
	}

	for _, goFile := range pkg.GoFiles {
		fileSet := token.NewFileSet() // positions are relative to fset
		file, err := parser.ParseFile(fileSet, goFile, nil, 0)
		if err != nil {
			return nil, fmt.Errorf("Analyzer.GetStructs.parser.ParseFile: %v\n", err)
		}

		ast.Inspect(file, visitFunc)
	}

	return structMap, nil
}

func (a *Analyzer) GetPath(key string, httpHandler *types.Func, result *ParseResult) (string, error) {
	path := key
	// TODO:
	return path, nil
}

func (a *Analyzer) GetMethods(key string, httpHandler *types.Func, result *ParseResult) ([]string, error) {
	// TODO:

	return make([]string, 0), nil
}

func (a *Analyzer) GetParameters(key string, httpHandler *types.Func, result *ParseResult) (map[string]*types.Type, error) {
	// TODO:

	return make(map[string]*types.Type), nil
}

func (a *Analyzer) GetResponseModel(httpHandler *types.Func, result *ParseResult) (string, error) {
	return httpHandler.Name(), nil
}
