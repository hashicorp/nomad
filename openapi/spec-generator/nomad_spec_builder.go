package main

import (
	"golang.org/x/tools/go/packages"
)

// newNomadSpecBuilder is a factory method for the NomadSpecBuilder struct
func newNomadSpecBuilder(analyzer *Analyzer, visitor *PackageVisitor, logger loggerFunc) *SpecBuilder {
	return &SpecBuilder{
		Visitor: visitor,
		logger:  logger,
	}
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

var nomadPackages = []*PackageConfig{
	{
		Config: packages.Config{
			Dir:  "../../api/",
			Mode: loadMode,
		},
		Pattern: ".",
	},
	{
		Config: packages.Config{
			Dir:  "../../command/agent/",
			Mode: loadMode,
		},
		Pattern: ".",
	},
	//{
	//	Config: packages.Config{
	//		Dir:  "../../client/structs/",
	//		Mode: loadMode,
	//	},
	//	Pattern: ".",
	//	Alias: "cstructs",
	//},
}
