package main

import (
	"fmt"
	"go/types"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
	"os"
	"strings"
)

type SSAnalyzer struct {
	PackageConfigs []*PackageConfig
	RuntimeTypes   map[string]*types.Type
	Logger         loggerFunc
	prog           *ssa.Program
}

type SSADebugOptions struct {
	writePackages bool
}

// TODO: Moved over from Analzyer. If we decide we need this will need to merge
// the two methods.
func (s *SSAnalyzer) buildProgram() error {
	loadedPkgs := make([]*packages.Package, 0)

	for i := 0; i < len(s.PackageConfigs); i++ {
		pkgs, err := packages.Load(&nomadPackages[i].Config, nomadPackages[i].Pattern)
		if err != nil {
			return err
		}
		for _, pkg := range pkgs {
			loadedPkgs = append(loadedPkgs, pkg)
		}
	}

	s.prog, _ = ssautil.AllPackages(loadedPkgs, ssa.NaiveForm) // to dump full program summary to stdout pass ssa.PrintPackages
	s.prog.Build()
	s.FilterRuntimeTypes()

	return nil
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
	var ssaPkgs []*ssa.Package
	s.prog, ssaPkgs = ssautil.AllPackages(allPkgs, ssa.PrintPackages) // to dump full program pass ssa.PrintPackages
	// Build SSA code for the whole program.
	s.prog.Build()

	if debugOptions.writePackages {
		for _, pkg := range ssaPkgs {
			if _, err := pkg.WriteTo(os.Stdout); err != nil {
				return err
			}
		}
	}

	s.RuntimeTypes = make(map[string]*types.Type)
	for _, runtimeType := range s.prog.RuntimeTypes() {
		if strings.Contains(runtimeType.String(), "github.com/hashicorp/nomad") && !strings.Contains(runtimeType.String(), "testclient") {
			// logger(runtimeType)
			if _, ok := s.RuntimeTypes[runtimeType.String()]; !ok {
				s.RuntimeTypes[runtimeType.String()] = &runtimeType
				methodSet := s.prog.MethodSets.MethodSet(runtimeType)
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

func (s *SSAnalyzer) GetMethodSets(runtimeType types.Type) *types.MethodSet {
	return s.prog.MethodSets.MethodSet(runtimeType)
}

func (s *SSAnalyzer) GetReturnTypeByHandlerName(funcName string) interface{} {
	for key, runtimeType := range s.RuntimeTypes {
		if !strings.Contains(key, "/api") {
			continue
		}
		// @@@@ Left off Here
		//s.prog.LookupMethod()
		//s.prog.MethodValue()
		s.Logger(funcName, ": ", key)
		methodSet := s.GetMethodSets(*runtimeType)
		for i := 0; i < methodSet.Len(); i++ {
			method := methodSet.At(i)
			methodString := method.String()
			s.Logger(methodString)
			if strings.Contains(methodString, "nomad") {
				s.Logger(methodString)
			}
			if strings.Contains(types.SelectionString(method, nil), funcName) {
				s.Logger("found ", funcName, "in", method.String())
				if method.Type().(*types.Signature).Results().Len() > 0 {
					resultType := method.Type().(*types.Signature).Results().At(0)
					s.Logger(resultType.Name())
					return runtimeType
				}
			}
		}
	}
	return nil
}

// TODO: Leaving for now
func (s *SSAnalyzer) FilterRuntimeTypes() {
	if s.RuntimeTypes == nil {
		s.RuntimeTypes = make(map[string]*types.Type)
	}
	for _, runtimeType := range s.prog.RuntimeTypes() {
		if strings.Contains(runtimeType.String(), "github.com/hashicorp/nomad") && !strings.Contains(runtimeType.String(), "testclient") {
			if _, ok := s.RuntimeTypes[runtimeType.String()]; !ok {
				s.RuntimeTypes[runtimeType.String()] = &runtimeType

			}
		}
	}
}

func (a *Analyzer) GetTypeByName(name string) types.Object {
	if obj, ok := a.typeObjects[name]; ok {
		return obj
	}
	return nil
}

func (a *Analyzer) FormatTypeName(pkgName, typeName string) string {
	return fmt.Sprintf("%s.%s", pkgName, typeName)
}
