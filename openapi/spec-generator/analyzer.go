package main

import (
	"go/types"
	"golang.org/x/tools/go/packages"
	"strings"
)

type HTTPProfile struct {
	IsResponseWriter bool // net/http.ResponseWriter
	IsRequest        bool // *net/http.Request
	IsHandler        bool // net/http.Handler
}

// Analyzer provides a number of static analysis helper functions.
type Analyzer struct{}

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
