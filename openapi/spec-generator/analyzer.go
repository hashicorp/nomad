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
	logger   loggerFunc
	Funcs    map[string]*FuncInfo
	files    []*ast.File
	fileSets []*token.FileSet
}

func (v *NomadNodeVisitor) DebugPrint() {
	for key, fn := range v.Funcs {
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
		// TODO: Do I want to constrain only on HTTP Handlers and/or functions
		// that either call the Handler or get called by the handler.
		var src string
		var err error
		if src, err = v.getSource(t); err != nil {
			v.logger(fmt.Errorf("VisitNode.getSourceL %v\n", err))
			return true
		}
		if v.Funcs == nil {
			v.Funcs = make(map[string]*FuncInfo)
		}

		// TODO: Handle receiver
		if _, ok := v.Funcs[t.Name.Name]; !ok {
			v.Funcs[t.Name.Name] = &FuncInfo{
				Fn:     t,
				Source: src,
			}
		} else {
			v.logger(fmt.Sprintf("unexpected duplicate function name: %s", t.Name.Name))
			return true
		}
	case *ast.ReturnStmt:
		for _, result := range t.Results {
			switch r := result.(type) {
			case *ast.Ident:
				if r != nil && r.Obj != nil {
					v.logger(r.Name)
				}
			case *ast.CallExpr:
				for _, a := range r.Args {
					switch at := a.(type) {
					case *ast.BasicLit:
						v.logger(at.Value)
					case *ast.Ident:
						v.logger(at.Name)
					}
				}
			default:
				v.logger(r)
			}
		}
	case *ast.BlockStmt:
		for _, stmt := range t.List {
			v.logger(stmt)
		}
	case *ast.BranchStmt:
		v.logger(t.Tok.String())
	}
	return true
}

func (v *NomadNodeVisitor) getSource(t *ast.FuncDecl) (string, error) {
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, v.GetActiveFileSet(), t.Body); err != nil {
		return "", err
	}
	return buf.String(), nil
}

type FuncInfo struct {
	Callers *ast.Node
	Fn      *ast.FuncDecl
	Source  string
}

type HTTPProfile struct {
	IsResponseWriter bool // net/http.ResponseWriter
	IsRequest        bool // *net/http.Request
	IsHandler        bool // net/http.Handler
}

// Analyzer provides a number of static analysis helper functions.
type Analyzer struct{}

func (a *Analyzer) GetFuncInfo(funcNames []string, pkg packages.Package) ([]*FuncInfo, error) {

	return nil, nil
}

func (a *Analyzer) analyzeHTTPProfile(tup *types.Tuple, result *HTTPProfile) *HTTPProfile {
	if tup == nil {
		return result
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

	return result
}

func (a *Analyzer) GetHttpHandlers(pkg *packages.Package) map[string]*types.Func {
	httpHandlers := make(map[string]*types.Func)
	for _, typeDef := range pkg.TypesInfo.Defs {
		if typeDef != nil {
			if typeDefFunc, ok := typeDef.(*types.Func); ok {
				if funcSignature, ok := typeDefFunc.Type().(*types.Signature); ok {
					// typeDefFunc.
					result := HTTPProfile{}

					a.analyzeHTTPProfile(funcSignature.Params(), &result)
					a.analyzeHTTPProfile(funcSignature.Results(), &result)

					if result.IsHandler || (result.IsResponseWriter && result.IsRequest) {
						httpHandlers[typeDefFunc.Name()] = typeDefFunc
					}
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
