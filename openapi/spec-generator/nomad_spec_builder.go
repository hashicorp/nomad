package main

import (
	"errors"
	"fmt"
	"github.com/getkin/kin-openapi/openapi3"
	"go/types"
	"golang.org/x/tools/go/packages"
)

func NewNomadSpecBuilder() *SpecBuilder {
	return &SpecBuilder{
		PathAdapters: []*SourceAdapter{
			{
				Parser: &PackageParser{
					Config: packages.Config{
						Dir:  "../../command/agent/",
						Mode: loadMode,
					},
					Pattern: ".",
				},
				Adapt: NomadPathAdapterFunc,
			},
		},
	}
}

func NomadPathAdapterFunc(spec *Spec, result *ParseResult) error {
	httpHandlers := spec.analyzer.GetHttpHandlers(result.Package)

	if len(httpHandlers) < 1 {
		return errors.New("NomadSpecBuilder.NomadPathAdapterFunc: no handlers found")
	}

	for key, httpHandler := range httpHandlers {
		path, err := spec.analyzer.GetPath(key, httpHandler, result)
		if err != nil {
			return fmt.Errorf("NomadSpecBuilder.NomadPathAdapter.GetPath: %v\n", err)
		}

		methods, err := spec.analyzer.GetMethods(key, httpHandler, result)
		if err != nil {
			return fmt.Errorf("NomadSpecBuilder.NomadPathAdapter.GetMethods: %v\n", err)
		}

		for _, method := range methods {
			operation := &openapi3.Operation{}
			params, err := spec.analyzer.GetParameters(key, httpHandler, result)
			if err != nil {
				fmt.Errorf("NomadSpecBuilder.NomadPathAdapter.GetParameters: %v\n", err)
			}
			for paramName, param := range params {
				if existing, ok := spec.Model.Components.Parameters[paramName]
				spec.Model.Components.Parameters[paramName] = convertParam(param)
				operation.AddParameter()
			}
			spec.Model.AddOperation(path, method, operation)
		}
	}

	return nil
}

func convertParam(param *types.Type) *openapi3.ParameterRef {

}

const loadMode = packages.NeedName |
	packages.NeedFiles |
	packages.NeedCompiledGoFiles |
	packages.NeedImports |
	packages.NeedDeps |
	packages.NeedExportsFile |
	packages.NeedTypes |
	packages.NeedSyntax |
	packages.NeedTypesInfo |
	packages.NeedTypesSizes |
	packages.NeedModule
