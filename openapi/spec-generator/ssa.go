package main

import (
	"go/types"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
	"os"
	"strings"
)

type SSAnalyzer struct {
	RuntimeTypes map[string]*types.Type
}

type SSADebugOptions struct {
	writePackages bool
}

func (s *SSAnalyzer) BuildProgram(debugOptions SSADebugOptions, logger func(args ...interface{})) error {
	allPkgs := make([]*packages.Package, 0)

	for i := 0; i < len(nomadPackages); i++ {
		pkgs, err := packages.Load(&nomadPackages[i].Config, nomadPackages[i].Pattern)
		if err != nil {
			return err
		}
		for _, pkg := range pkgs {
			allPkgs = append(allPkgs, pkg)
		}
	}

	// Create SSA packages for well-typed packages and their dependencies.
	prog, ssaPkgs := ssautil.AllPackages(allPkgs, ssa.NaiveForm) // to dump full program pass ssa.PrintPackages
	// Build SSA code for the whole program.
	prog.Build()

	if debugOptions.writePackages {
		for _, pkg := range ssaPkgs {
			if _, err := pkg.WriteTo(os.Stdout); err != nil {
				return err
			}
		}
	}

	s.RuntimeTypes = make(map[string]*types.Type)
	for _, runtimeType := range prog.RuntimeTypes() {
		if strings.Contains(runtimeType.String(), "github.com/hashicorp/nomad") && !strings.Contains(runtimeType.String(), "testclient") {
			// logger(runtimeType)
			if _, ok := s.RuntimeTypes[runtimeType.String()]; !ok {
				s.RuntimeTypes[runtimeType.String()] = &runtimeType
				methodSet := prog.MethodSets.MethodSet(runtimeType)
				filter := false
				for i := 0; i < methodSet.Len(); i++ {
					method := methodSet.At(i)
					methodString := types.SelectionString(method, nil)
					if strings.Contains(methodString, "Job") {
						filter = true
					}
				}
				if filter {
					logger("runtimeType: ", runtimeType)
					for i := 0; i < methodSet.Len(); i++ {
						method := methodSet.At(i)
						methodString := types.SelectionString(method, nil)
						logger(methodString)
						if method.Type().(*types.Signature).Results().Len() > 0 {
							resultType := method.Type().(*types.Signature).Results().At(0)
							logger(resultType)
						}
					}
				}
			}
		}
	}

	return nil
}
