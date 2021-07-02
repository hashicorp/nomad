package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"go/types"
	"golang.org/x/tools/go/packages"
	"strings"
)

type loggerFunc func(args ...interface{})

type NomadNodeVisitor struct {
	HandlerFuncs map[string]*HandlerFuncInfo
	packages     map[string]*packages.Package
	analyzer     *Analyzer
	logger       loggerFunc
	files        []*ast.File
	fileSets     []*token.FileSet
}

func (v *NomadNodeVisitor) SetPackages(pkgs []*packages.Package) error {
	if v.packages == nil {
		v.packages = make(map[string]*packages.Package)
	}

	if v.HandlerFuncs == nil {
		v.HandlerFuncs = make(map[string]*HandlerFuncInfo)
	}

	for _, pkg := range pkgs {
		if _, ok := v.packages[pkg.Name]; ok {
			return fmt.Errorf(fmt.Sprintf("NomadVisitor.SetPackages: Package %s alread exists", pkg.Name))
		}

		v.packages[pkg.Name] = pkg
		v.SetActiveFileSet(pkg.Fset)

		if err := v.loadHandlers(pkg); err != nil {
			return err
		}
	}
	return nil
}

func (v *NomadNodeVisitor) loadHandlers(pkg *packages.Package) error {
	handlers := v.analyzer.GetHttpHandlers(pkg)
	for key, handler := range handlers {
		if _, ok := v.HandlerFuncs[v.getTypesFuncNameWithReceiver(handler)]; ok {
			return fmt.Errorf("NomadVisitor.loadHandlers package %s alread exists", key)
		}

		v.HandlerFuncs[v.getTypesFuncNameWithReceiver(handler)] = &HandlerFuncInfo{
			Func:     handler,
			logger:   v.logger,
			analyzer: v.analyzer,
			fileSet:  v.GetActiveFileSet(),
		}
	}
	return nil
}

func (v *NomadNodeVisitor) DebugPrint() {
	for key, fn := range v.HandlerFuncs {
		v.logger(fmt.Sprintf("%s: %s\n\n", key, fn.Source))
	}
}

func (v *NomadNodeVisitor) Files() []*ast.File {
	return v.files
}

func (v *NomadNodeVisitor) FileSets() []*token.FileSet {
	return v.fileSets
}

func (v *NomadNodeVisitor) SetActiveFileSet(fileSet *token.FileSet) {
	v.fileSets = append(v.fileSets, fileSet)
}

func (v *NomadNodeVisitor) GetActiveFileSet() *token.FileSet {
	if len(v.fileSets) < 1 {
		return nil
	}
	return v.fileSets[len(v.fileSets)-1]
}

func (v *NomadNodeVisitor) VisitNode(node ast.Node) bool {
	switch t := node.(type) {
	case *ast.FuncDecl:
		var src string
		var err error
		if src, err = v.analyzer.GetSource(t.Body, v.GetActiveFileSet()); err != nil {
			v.logger(fmt.Errorf("VisitNode.getSourceL %v\n", err))
			return true
		}
		name := v.getFuncDeclNameWithReceiver(t)
		// If not a handler then don't add the func
		if _, ok := v.HandlerFuncs[name]; !ok {
			return true
		}

		// v.logger(fmt.Sprintf("Found HandlerFuncInfo for %s", name))

		params := ""
		for _, param := range t.Type.Params.List {
			params = fmt.Sprintf("%s|%s ", param.Names[0].Name, param.Type)
		}
		src = fmt.Sprintf("%s - %s", params, src)

		if _, ok := v.HandlerFuncs[name]; !ok {
			panic(fmt.Sprintf(fmt.Sprintf("VisitNode failed to resolve HandlerFuncInfo for %s", name)))
		} else {
			funcInfo := v.HandlerFuncs[name]
			funcInfo.FuncDecl = t
			funcInfo.Source = src
			ast.Inspect(t, funcInfo.visitFunc)
			if err = funcInfo.processVisitResults(); err != nil {
				panic(fmt.Errorf(fmt.Sprintf("FuncInfo.processVisitResults failed for %s", name), err))
			}
		}
	}
	return true
}

func (v *NomadNodeVisitor) getFuncDeclNameWithReceiver(t *ast.FuncDecl) string {
	name := t.Name.Name

	if t.Recv != nil {
		var recv string
		if stex, ok := t.Recv.List[0].Type.(*ast.StarExpr); ok {
			recv = stex.X.(*ast.Ident).Name
		} else if id, ok := t.Recv.List[0].Type.(*ast.Ident); ok {
			recv = id.Name
		}

		name = fmt.Sprintf("%s.%s", recv, name)
	}
	return name
}

func (v *NomadNodeVisitor) getTypesFuncNameWithReceiver(handler *types.Func) string {
	name := handler.Name()
	signature := handler.Type().(*types.Signature)
	if recv := signature.Recv(); recv != nil {
		recvSegments := strings.Split(recv.Type().String(), "/")
		recvName := recvSegments[len(recvSegments)-1]
		runes := []rune(recvName)
		recvName = string(runes[strings.Index(recvName, ".")+1:])
		name = fmt.Sprintf("%s.%s", recvName, name)
	}
	return name
}

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
	Params           []*ParamInfo
	ResponseSchema   *ast.Expr
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
	if err := f.resolveResponseSchema(); err != nil {
		return err
	}
	return nil
}

func (f *HandlerFuncInfo) resolveResponseSchema() error {
	if len(f.returnStatements) < 1 {
		return fmt.Errorf("HandlerFuncInfo.resolveResponseSchema: no return statement found")
	}

	// If more than one return statement returns a non-nil value then this is a
	// FooSpecificRequest Handlers. Happy Accident!!!
	var responseSchema ast.Expr
	var err error
	if responseSchema, err = f.getFinalReturnType(); err != nil {
		return err
	}

	if responseSchema == nil {
		if err = f.moveToPathSwitchHandler(); err != nil {
			return err
		}
		return nil
	}

	f.ResponseSchema = &responseSchema

	// DEBUG
	src, err := f.analyzer.GetSource(responseSchema, f.fileSet)
	if err != nil {
		return fmt.Errorf("HandlerFuncInfo.resolveResponseSchema: cannot render source")
	}
	f.logger(fmt.Sprintf("%s.finalReturn.source: %s", f.Name(), src))

	return nil
}

func (f *HandlerFuncInfo) getFinalReturnType() (ast.Expr, error) {
	finalReturn := f.returnStatements[len(f.returnStatements)-1]
	if finalReturn == nil {
		return nil, fmt.Errorf("HandlerFuncInfo.getFinalReturnType: finalReturn does not exist")
	}

	return finalReturn.Results[0], nil
}

func (f *HandlerFuncInfo) moveToPathSwitchHandler() error {
	for _, retStmt := range f.returnStatements {
		src, err := f.analyzer.GetSource(&retStmt, f.fileSet)
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
type Analyzer struct{}

func (a *Analyzer) GetSource(elem interface{}, fileSet *token.FileSet) (string, error) {
	// Try the happy path first
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fileSet, elem); err != nil {
		return "", err
	} else {
		return buf.String(), nil
	}

	switch elem.(type) {
	case *ast.SelectorExpr:
		selector := elem.(*ast.SelectorExpr)
		switch selector.X.(type) {
		case *ast.Ident:
			ident := selector.X.(*ast.Ident)
			if ident.Name == "out" {
				valueSpecSelector := ident.Obj.Decl.(*ast.ValueSpec).Type.(*ast.SelectorExpr)
				packageName := valueSpecSelector.X.(*ast.Ident).Name
				structName := valueSpecSelector.Sel.Name
				return fmt.Sprintf("%s.%s", packageName, structName), nil
			}
		}
	case *ast.Expr:
		expr := elem.(*ast.Expr)
		fmt.Println(expr)
		return "unknown", nil
		//switch expr.(type) {
		//case *ast.Ident:
		//	ident := selector.X.(*ast.Ident)
		//	if ident.Name == "out" {
		//		valueSpecSelector := ident.Obj.Decl.(*ast.ValueSpec).Type.(*ast.SelectorExpr)
		//		packageName := valueSpecSelector.X.(*ast.Ident).Name
		//		structName := valueSpecSelector.Sel.Name
		//		return fmt.Sprintf("%s.%s", packageName, structName), nil
		//	}
		//	panic("Unhandled SelectorExpr")
		//}
	}

	return "unhandled", nil
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
