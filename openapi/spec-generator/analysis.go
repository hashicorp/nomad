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

func analyzeHTTPProfile(tup *types.Tuple, result *HTTPProfile) *HTTPProfile {
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

func GetHttpHandlers(pkg *packages.Package) map[string]*types.Func {
	httpHandlers := make(map[string]*types.Func)

	for _, typeDef := range pkg.TypesInfo.Defs {
		if typeDef != nil {
			if typesFunc, ok := typeDef.(*types.Func); ok {
				if funcSignature, ok := typesFunc.Type().(*types.Signature); ok {
					result := HTTPProfile{}

					analyzeHTTPProfile(funcSignature.Params(), &result)
					analyzeHTTPProfile(funcSignature.Results(), &result)

					if result.IsHandler || (result.IsResponseWriter && result.IsRequest) {
						httpHandlers[typeDef.String()] = typesFunc
					}
				}
			}
		}
	}

	return httpHandlers
}

func GetPath(key string, httpHandler *types.Func) (string, error) {
	path := "unknown"

	return path, nil
}
