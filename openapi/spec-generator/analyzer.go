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

//type HandlerFuncInfo interface {
//	GetPath() string
//	GetSource() string
//	GetMethodInfos() []*MethodInfo
//}

//type MethodInfo interface {
//
//}

type HandlerFuncInfo struct {
	Path                 string
	Source               string
	PackageName          string
	Func                 *types.Func
	FuncDecl             *ast.FuncDecl
	ResponseTypeFullName string
	ResponseType         *ast.TypeSpec
	Structs              map[string]*ast.TypeSpec
	Params               []*ParamInfo
	ResponseHeaders      []*HeaderInfo
	logger               loggerFunc
	analyzer             *Analyzer
	fileSet              *token.FileSet
	returnStatements     []*ast.ReturnStmt
}

func (h *HandlerFuncInfo) Name() string {
	return h.Func.Name()
}

func (h *HandlerFuncInfo) IsHelperFunction() bool {
	return h.ResponseType == nil
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
	if err := f.ResolveReturnType(); err != nil {
		return err
	}
	return nil
}

func (f *HandlerFuncInfo) ResolveReturnType() error {
	if len(f.returnStatements) < 1 {
		return fmt.Errorf("HandlerFuncInfo.resolveResponseType: no return statement found")
	}

	outTypeName := ""

	outVisitor := func(node ast.Node) bool {
		switch t := node.(type) {
		case *ast.GenDecl:
			if t.Tok != token.VAR {
				return true
			}
			for _, spec := range t.Specs {
				if value, ok := spec.(*ast.ValueSpec); ok {
					for _, name := range value.Names {
						if name.Name == "out" {
							selExpr := value.Type.(*ast.SelectorExpr)
							outTypeName = fmt.Sprintf("%s.%s", selExpr.X.(*ast.Ident).Name, selExpr.Sel.Name)
							return true
						}
					}
				}
			}
		}
		return true
	}

	ast.Inspect(f.FuncDecl, outVisitor)

	if len(outTypeName) > 0 {
		var ok bool
		if f.ResponseType, ok = f.Structs[outTypeName]; ok {
			f.ResponseTypeFullName = outTypeName
			return nil
		}
	}

	finalReturn := f.returnStatements[len(f.returnStatements)-1]
	if finalReturn == nil {
		return fmt.Errorf("HandlerFuncInfo.ResolveReturnType: finalReturn does not exist")
	}

	result := finalReturn.Results[0]
	returnParamName, err := f.analyzer.GetSource(result, f.fileSet)
	if err != nil {
		return err
	}

	// gets the out type from the source string. Heavily dependent on the convention
	// of the out parameter being named out.
	// Find the variable name
	sourceParts := strings.Split(f.Source, "var out ")

	// This is a handler switch, so return nil
	// TODO: handy way to find and parse handler switches
	if len(sourceParts) < 2 {
		return nil
	}

	// find the type of the variable
	outTypeName = strings.Split(sourceParts[1], " ")[0]
	// strip off everything after the variable declaration
	outTypeName = strings.Split(outTypeName, "\n")[0]

	var ok bool

	// If it returns nil, it should be a Foo specific handler
	if returnParamName == "nil" {
		return nil
	} else if strings.Index(returnParamName, "out.") > -1 { // handle return of out type field
		var outType *ast.TypeSpec
		pkgName := ""
		// if the outTypeName includes a package name get it so we can prepend later.
		if strings.Index(outTypeName, ".") > -1 {
			pkgName = strings.Split(outTypeName, ".")[0]
		}
		if outType, ok = f.Structs[outTypeName]; !ok {

			return fmt.Errorf("HandlerFuncInfo.ResolveReturnType: cannot resolve type for name %s", outTypeName)
		}
		returnTypeFieldName := strings.Split(returnParamName, "out.")[1]
		for _, field := range outType.Type.(*ast.StructType).Fields.List {
			fieldTypeName := "unknown"
			switch t := field.Type.(type) {
			case *ast.Ident:
				fieldTypeName = field.Type.(*ast.Ident).Name
			case *ast.ArrayType:
				expr, ok := t.Elt.(*ast.StarExpr)
				if ok {
					fieldTypeName = expr.X.(*ast.Ident).Name
				} else {
					fieldTypeName = t.Elt.(*ast.Ident).Name
				}
			case *ast.MapType:
				expr, ok := t.Value.(*ast.StarExpr)
				if ok {
					fieldTypeName = expr.X.(*ast.Ident).Name
				} else {
					fieldTypeName = t.Value.(*ast.Ident).Name
				}
			case *ast.StructType:
				fieldTypeName = field.Names[0].Name

			case *ast.StarExpr:
				fieldTypeName = t.X.(*ast.Ident).Name
			}

			if fieldTypeName == returnTypeFieldName {
				if len(pkgName) > 0 {
					fieldTypeName = fmt.Sprintf("%s.%s", pkgName, fieldTypeName)
				}
				f.ResponseType = f.Structs[fieldTypeName]
			}
		}

	} else if returnParamName == "out" { // handle direct return of out type
		if f.ResponseType, ok = f.Structs[outTypeName]; !ok {
			return fmt.Errorf("HandlerFuncInfo.ResolveReturnType: cannot resolve type for name %s", outTypeName)
		}
	} else {
		// handle things not named out
		return fmt.Errorf("ResolveReturnTypeName.Unhandled %s", returnParamName)
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
				structMap[fmt.Sprintf(fmtString, typeSpec.Name.Name)] = typeSpec
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
