package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"golang.org/x/tools/go/packages"
)

type loggerFunc func(args ...interface{})

type DebugOptions struct {
	showSource       bool
	showHelpers      bool
	showHandlers     bool
	showReturnSource bool
}

var defaultDebugOptions = DebugOptions{
	showHandlers:     true,
	showSource:       true,
	showHelpers:      false,
	showReturnSource: false,
}

type NomadPackageVisitor struct {
	HandlerAdapters map[string]*HandlerFuncAdapter
	Structs         map[string]*ast.TypeSpec
	packages        map[string]*packages.Package
	activePackage   *packages.Package
	analyzer        *Analyzer
	logger          loggerFunc
	fileSets        []*token.FileSet
	debugOptions    DebugOptions
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
		v.analyzer.typesInfos = append(v.analyzer.typesInfos, pkg.TypesInfo)

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

			ast.Inspect(file, v.VisitFile)
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

	// TODO: Add comprehensive debug switches
	for key, fn := range v.HandlerAdapters {
		src, err := fn.GetSource()
		if err != nil {
			continue
		}
		if v.debugOptions.showHelpers && fn.IsHelperFunction() {
			if v.debugOptions.showSource {
				v.logger(fmt.Sprintf("%s: may be a helper function - Response Type: %s\n - Params/Source: %s", key, "unknown", src))
			} else if v.debugOptions.showReturnSource {
				v.logger(fmt.Sprintf("%s: may be a helper function - return source: %s", key, fn.debugReturnSource(0)))
			} else {
				v.logger(fmt.Sprintf("%s: may be a helper function - Response Type: %s", key, "unknown"))
			}
		} else if v.debugOptions.showHandlers {
			if v.debugOptions.showSource {
				v.logger(fmt.Sprintf("%s: Response Type: %s\n - Params/Source: %s", key, fn.Path, src))
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

func (v *NomadPackageVisitor) VisitFile(node ast.Node) bool {
	switch t := node.(type) {
	case *ast.FuncDecl:
		name := fmt.Sprintf("%s.%s", v.activePackage.Name, t.Name.Name)
		// If not a handler then don't add the func
		if _, ok := v.HandlerAdapters[name]; !ok {
			return true
		}

		if _, ok := v.HandlerAdapters[name]; !ok {
			panic(fmt.Sprintf(fmt.Sprintf("VisitFile failed to resolve HandlerFuncAdapter for %s", name)))
		} else {
			adapter := v.HandlerAdapters[name]
			adapter.FuncDecl = t
			ast.Inspect(t, adapter.visitFunc)
			if err := adapter.processVisitResults(); err != nil {
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
