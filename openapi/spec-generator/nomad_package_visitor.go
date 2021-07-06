package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"golang.org/x/tools/go/packages"
)

type loggerFunc func(args ...interface{})

type NomadPackageVisitor struct {
	HandlerAdapters map[string]*HandlerFuncAdapter
	Structs         map[string]*ast.TypeSpec
	packages        map[string]*packages.Package
	activePackage   *packages.Package
	analyzer        *Analyzer
	logger          loggerFunc
	fileSets        []*token.FileSet
}

func (v *NomadPackageVisitor) GetHandlerAdapters() map[string]*HandlerFuncAdapter {
	return v.HandlerAdapters
}

func (v *NomadPackageVisitor) VisitPackages(pkgs []*packages.Package) error {
	var err error
	if v.analyzer == nil {
		v.analyzer = &Analyzer{}
	}

	if v.packages == nil {
		v.packages = make(map[string]*packages.Package)
	}

	// Must load all structs from all packages BEFORE loading Handlers.
	if v.Structs == nil {
		v.Structs = make(map[string]*ast.TypeSpec)
	}

	for _, pkg := range pkgs {
		v.activePackage = pkg
		if err = v.mergeTypesInfo(pkg); err != nil {
			return err
		}
		// TODO: Can we stop this now that we are using TypesInfo?
		if err = v.loadStructs(pkg); err != nil {
			return err
		}
	}

	// Now load all handlers
	if v.HandlerAdapters == nil {
		v.HandlerAdapters = make(map[string]*HandlerFuncAdapter)
	}

	for _, pkg := range pkgs {
		if _, ok := v.packages[pkg.Name]; ok {
			return fmt.Errorf(fmt.Sprintf("NomadVisitor.VisitPackages: Package %s alread exists", pkg.Name))
		}

		v.packages[pkg.Name] = pkg
		v.activePackage = pkg
		v.SetActiveFileSet(pkg.Fset)

		if err := v.loadHandlers(); err != nil {
			return err
		}
	}

	for _, pkg := range pkgs {
		for _, goFile := range pkg.GoFiles {
			fileSet := token.NewFileSet() // positions are relative to fset
			file, err := parser.ParseFile(fileSet, goFile, nil, 0)
			if err != nil {
				return fmt.Errorf("PackageParser.parseGoFile: %v\n", err)
			}

			ast.Inspect(file, v.VisitNode)
		}
	}
	return nil
}

func (v *NomadPackageVisitor) mergeTypesInfo(pkg *packages.Package) error {
	if v.analyzer.TypesInfo == nil {
		v.analyzer.TypesInfo = pkg.TypesInfo
	} else {
		// TODO: Can this be genericized to helper func with map[iface]iface?
		for key, value := range pkg.TypesInfo.Types {
			if _, ok := v.analyzer.TypesInfo.Types[key]; ok {
				return fmt.Errorf("NomadPackageVisitor.VisitPackages.mergeTypesInfo.TypesInfo: key %s already exists", key)
			}
			v.analyzer.TypesInfo.Types[key] = value
		}
		for key, value := range pkg.TypesInfo.Defs {
			if _, ok := v.analyzer.TypesInfo.Defs[key]; ok {
				return fmt.Errorf("NomadPackageVisitor.VisitPackages.mergeTypesInfo.Defs: key %s already exists", key)
			}
			v.analyzer.TypesInfo.Defs[key] = value
		}
		for key, value := range pkg.TypesInfo.Implicits {
			if _, ok := v.analyzer.TypesInfo.Implicits[key]; ok {
				return fmt.Errorf("NomadPackageVisitor.VisitPackages.mergeTypesInfo.Implicits: key %s already exists", key)
			}
			v.analyzer.TypesInfo.Implicits[key] = value
		}
		for key, value := range pkg.TypesInfo.Scopes {
			if _, ok := v.analyzer.TypesInfo.Scopes[key]; ok {
				return fmt.Errorf("NomadPackageVisitor.VisitPackages.mergeTypesInfo: key %s already exists", key)
			}
			v.analyzer.TypesInfo.Scopes[key] = value
		}
		for key, value := range pkg.TypesInfo.Uses {
			if _, ok := v.analyzer.TypesInfo.Uses[key]; ok {
				return fmt.Errorf("NomadPackageVisitor.VisitPackages.mergeTypesInfo: key %s already exists", key)
			}
			v.analyzer.TypesInfo.Uses[key] = value
		}
		for key, value := range pkg.TypesInfo.Selections {
			if _, ok := v.analyzer.TypesInfo.Selections[key]; ok {
				return fmt.Errorf("NomadPackageVisitor.VisitPackages.mergeTypesInfo: key %s already exists", key)
			}
			v.analyzer.TypesInfo.Selections[key] = value
		}
	}
	return nil
}

func (v *NomadPackageVisitor) loadHandlers() error {
	handlers := v.analyzer.GetHttpHandlers(v.activePackage)
	for key, handler := range handlers {
		if _, ok := v.HandlerAdapters[key]; ok {
			return fmt.Errorf("NomadVisitor.loadHandlers package %s already exists", key)
		}

		v.HandlerAdapters[key] = &HandlerFuncAdapter{
			Package:  v.activePackage,
			Func:     handler,
			Structs:  v.Structs,
			logger:   v.logger,
			analyzer: v.analyzer,
			fileSet:  v.GetActiveFileSet(),
		}
	}
	return nil
}

func (v *NomadPackageVisitor) DebugPrint() {
	// setting up debug options for extraction.
	showSource := false
	showHelpers := false
	showHandlers := true
	// TODO: Add comprehensive debug switches
	for key, fn := range v.HandlerAdapters {
		if fn.IsHelperFunction() && showHelpers {
			if showSource {
				v.logger(fmt.Sprintf("%s: is a helper function - Response Type: %s\n - Params/Source: %s", key, "unknown", fn.Source))
			} else {

				v.logger(fmt.Sprintf("%s: is a helper function - Response Type: %s", key, "unknown"))
			}
		} else if showHandlers {
			if showSource {
				v.logger(fmt.Sprintf("%s: Response Type: %s\n - Params/Source: %s", key, fn.Path, fn.Source))
			} else {
				if fn.ResponseType == nil {
					// v.logger(fmt.Sprintf("%s: Response Type: %s", key, "unknown"))
				} else {
					v.logger(fmt.Sprintf("%s: Response Type: %s", key, fn.ResponseType.Name))
				}
			}
		}
	}
}

func (v *NomadPackageVisitor) SetActiveFileSet(fileSet *token.FileSet) {
	v.fileSets = append(v.fileSets, fileSet)
}

func (v *NomadPackageVisitor) GetActiveFileSet() *token.FileSet {
	if len(v.fileSets) < 1 {
		return nil
	}
	return v.fileSets[len(v.fileSets)-1]
}

func (v *NomadPackageVisitor) VisitNode(node ast.Node) bool {
	switch t := node.(type) {
	case *ast.FuncDecl:
		name := fmt.Sprintf("%s.%s", v.activePackage.Name, t.Name.Name)
		// If not a handler then don't add the func
		if _, ok := v.HandlerAdapters[name]; !ok {
			return true
		}

		var err error
		var src string
		if src, err = v.analyzer.GetSource(t.Body, v.GetActiveFileSet()); err != nil {
			v.logger(fmt.Errorf("VisitNode.analayzer.GetSource %v\n", err))
			return true
		}

		if _, ok := v.HandlerAdapters[name]; !ok {
			panic(fmt.Sprintf(fmt.Sprintf("VisitNode failed to resolve HandlerFuncAdapter for %s", name)))
		} else {
			adapter := v.HandlerAdapters[name]
			adapter.FuncDecl = t
			adapter.Source = src
			ast.Inspect(t, adapter.visitFunc)
			if err = adapter.processVisitResults(); err != nil {
				panic(fmt.Errorf(fmt.Sprintf("FuncInfo.processVisitResults failed for %s", name), err))
			}
		}
	}
	return true
}

func (v *NomadPackageVisitor) loadStructs(pkg *packages.Package) error {
	var structs map[string]*ast.TypeSpec
	var err error
	if structs, err = v.analyzer.GetStructs(pkg); err != nil {
		return err
	}

	for _, s := range structs {
		key := fmt.Sprintf("%s.%s", pkg.Name, s.Name)
		if _, ok := v.Structs[key]; ok {
			return fmt.Errorf("NomadPackageVisitor.loadStructs found duplicate key: %s", key)
		}
		v.Structs[key] = s
	}

	return nil
}
