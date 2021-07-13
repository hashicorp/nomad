package main

import (
	"fmt"
	"github.com/getkin/kin-openapi/openapi3"
	"go/ast"
	"go/parser"
	"go/token"
	"golang.org/x/tools/go/packages"
)

type loggerFunc func(args ...interface{})

type DebugOptions struct {
	printSource       bool
	printHelpers      bool
	printHandlers     bool
	printReturnSource bool
	filterByMethods   []string
	printDefs         bool
	printVariables    bool
	printSchemaRefs   bool
}

var defaultDebugOptions = DebugOptions{
	printHandlers:  true,
	printVariables: true,
}

func NewNomadPackageVisitor(analyzer *Analyzer, logger loggerFunc, options DebugOptions) PackageVisitor {
	visitor := &NomadPackageVisitor{
		analyzer:         analyzer,
		logger:           logger,
		debugOptions:     options,
		schemaRefAdapter: NewSchemaRefAdapter(analyzer),
	}

	return visitor
}

type NomadPackageVisitor struct {
	handlerAdapters  map[string]*HandlerFuncAdapter
	schemaRefAdapter *SchemaRefAdapter
	analyzer         *Analyzer
	activePackage    *packages.Package
	logger           loggerFunc
	fileSets         []*token.FileSet
	debugOptions     DebugOptions
}

func (v *NomadPackageVisitor) SchemaRefs() map[string]*openapi3.SchemaRef {
	return v.schemaRefAdapter.SchemaRefs
}

func (v *NomadPackageVisitor) ParameterRefs() map[string]*openapi3.ParameterRef {
	return nil
}

func (v *NomadPackageVisitor) HeaderRefs() map[string]*openapi3.HeaderRef {
	return nil
}

func (v *NomadPackageVisitor) RequestBodyRefs() map[string]*openapi3.RequestBodyRef {
	return nil
}

func (v *NomadPackageVisitor) CallbackRefs() map[string]*openapi3.CallbackRef {
	return nil
}

func (v *NomadPackageVisitor) ResponseRefs() map[string]*openapi3.ResponseRef {
	return nil
}

func (v *NomadPackageVisitor) HandlerAdapters() map[string]*HandlerFuncAdapter {
	return v.handlerAdapters
}

func (v *NomadPackageVisitor) Analyzer() *Analyzer {
	return v.analyzer
}

func (v *NomadPackageVisitor) GetHandlerAdapters() map[string]*HandlerFuncAdapter {
	return v.handlerAdapters
}

func (v *NomadPackageVisitor) VisitPackages() error {
	// Load all handlers
	if v.handlerAdapters == nil {
		v.handlerAdapters = make(map[string]*HandlerFuncAdapter)
	}

	for _, pkg := range v.analyzer.Packages {
		v.activePackage = pkg
		v.SetActiveFileSet(pkg.Fset)

		if err := v.loadHandlers(); err != nil {
			return err
		}
	}

	for _, pkg := range v.analyzer.Packages {
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
		// Useful for Debug and Tests
		isTarget := false
		for _, h := range v.debugOptions.filterByMethods {
			if key == h {
				isTarget = true
			}
		}

		if !isTarget {
			continue
		}

		if _, ok := v.handlerAdapters[key]; ok {
			return fmt.Errorf("NomadVisitor.loadHandlers package %s already exists", key)
		}

		v.handlerAdapters[key] = &HandlerFuncAdapter{
			Package:          v.activePackage,
			Func:             handler,
			logger:           v.logger,
			analyzer:         v.analyzer,
			schemaRefAdapter: v.schemaRefAdapter,
			fileSet:          v.GetActiveFileSet(),
		}
	}
	return nil
}

func (v *NomadPackageVisitor) DebugPrint() {

	// TODO: Add comprehensive debug switches
	for key, fn := range v.handlerAdapters {
		// TODO: figure out why this is ever possible. Feels like a race condition.
		if fn.FuncDecl == nil {
			continue
		}
		src, err := fn.GetSource()
		if err != nil {
			continue
		}
		if v.debugOptions.printHandlers {
			if v.debugOptions.printSource {
				v.logger(fmt.Sprintf("%s: Response Type: %s\n - Params/Source: %s", key, fn.GetPath(), src))
			} else {
				retSchema, _ := fn.GetReturnSchema()
				if retSchema == nil {
					// v.logger(fmt.Sprintf("%s: Response Type: %s", key, "unknown"))
				} else {
					v.logger(fmt.Sprintf("%s: Response Type: %#v", key, retSchema.Value))
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
		name := v.analyzer.FormatTypeName(v.activePackage.Name, t.Name.Name)
		// If not a handler then don't add the func
		if _, ok := v.handlerAdapters[name]; !ok {
			return true
		}

		adapter := v.handlerAdapters[name]
		if t == nil {
			panic("t is nil for " + name)
		}
		adapter.FuncDecl = t
		adapter.Cfg = v.analyzer.GetControlFlowGraph(adapter.Func, adapter.FuncDecl)

		if err := adapter.visitHandlerFunc(); err != nil {
			panic(fmt.Errorf(fmt.Sprintf("FuncInfo.visitHandlerFunc failed for %s", name), err))
		}
	}
	return true
}
