package main

import (
	"errors"
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
	httpHandlers := GetHttpHandlers(result.Package)

	if len(httpHandlers) < 1 {
		return errors.New("NomadSpecBuilder.NomadPathAdapterFunc: no handlers found")
	}

	for key, httpHandler := range httpHandlers {
		spec.Model.AddOperation(GetPath(key, httpHandler), "TODO", nil)
	}

	return nil
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
