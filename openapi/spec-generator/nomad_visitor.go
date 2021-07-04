package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"golang.org/x/tools/go/packages"
	"strings"
)

type loggerFunc func(args ...interface{})

type NomadNodeVisitor struct {
	HandlerAdapters map[string]*HandlerFuncInfo
	Structs         map[string]*ast.TypeSpec
	packages        map[string]*packages.Package
	activePackage   *packages.Package
	analyzer        *Analyzer
	logger          loggerFunc
	fileSets        []*token.FileSet
}

func (v *NomadNodeVisitor) GetHandlerInfos() map[string]*HandlerFuncInfo {
	return v.HandlerAdapters
}

func (v *NomadNodeVisitor) VisitPackages(pkgs []*packages.Package) error {
	if v.packages == nil {
		v.packages = make(map[string]*packages.Package)
	}

	// Must load all structs from all packages BEFORE loading Handlers.
	if v.Structs == nil {
		v.Structs = make(map[string]*ast.TypeSpec)
	}

	for _, pkg := range pkgs {
		v.activePackage = pkg
		if err := v.loadStructs(pkg); err != nil {
			return err
		}
	}

	// Now load all handlers
	if v.HandlerAdapters == nil {
		v.HandlerAdapters = make(map[string]*HandlerFuncInfo)
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

func (v *NomadNodeVisitor) loadHandlers() error {
	handlers := v.analyzer.GetHttpHandlers(v.activePackage)
	for key, handler := range handlers {
		if _, ok := v.HandlerAdapters[v.getTypesFuncNameWithReceiver(handler)]; ok {
			return fmt.Errorf("NomadVisitor.loadHandlers package %s alread exists", key)
		}

		v.HandlerAdapters[v.getTypesFuncNameWithReceiver(handler)] = &HandlerFuncInfo{
			PackageName: v.activePackage.Name,
			Func:        handler,
			Structs:     v.Structs,
			logger:      v.logger,
			analyzer:    v.analyzer,
			fileSet:     v.GetActiveFileSet(),
		}
	}
	return nil
}

func (v *NomadNodeVisitor) DebugPrint() {
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
				v.logger(fmt.Sprintf("%s: Response Type: %s\n - Params/Source: %s", key, fn.ResponseTypeFullName, fn.Source))
			} else {

				v.logger(fmt.Sprintf("%s: Response Type: %s", key, fn.ResponseTypeFullName))
			}
		}
	}
}

func (v *NomadNodeVisitor) SetActiveFileSet(fileSet *token.FileSet) {
	v.fileSets = append(v.fileSets, fileSet)
}

func (v *NomadNodeVisitor) GetActiveFileSet() *token.FileSet {
	if len(v.fileSets) < 1 {
		return nil
	}
	return v.fileSets[len(v.fileSets)-1]
}

func (v *NomadNodeVisitor) VisitNode(node ast.Node) bool {
	switch t := node.(type) {
	case *ast.FuncDecl:
		var src string
		var err error
		if src, err = v.analyzer.GetSource(t.Body, v.GetActiveFileSet()); err != nil {
			v.logger(fmt.Errorf("VisitNode.analayzer.GetSource %v\n", err))
			return true
		}
		name := v.getFuncDeclNameWithReceiver(t)
		// If not a handler then don't add the func
		if _, ok := v.HandlerAdapters[name]; !ok {
			return true
		}

		params := ""
		for _, param := range t.Type.Params.List {
			params = fmt.Sprintf("%s|%s ", param.Names[0].Name, param.Type)
		}
		src = fmt.Sprintf("%s - %s", params, src)

		if _, ok := v.HandlerAdapters[name]; !ok {
			panic(fmt.Sprintf(fmt.Sprintf("VisitNode failed to resolve HandlerFuncInfo for %s", name)))
		} else {
			funcInfo := v.HandlerAdapters[name]
			funcInfo.FuncDecl = t
			funcInfo.Source = src
			ast.Inspect(t, funcInfo.visitFunc)
			if err = funcInfo.processVisitResults(); err != nil {
				panic(fmt.Errorf(fmt.Sprintf("FuncInfo.processVisitResults failed for %s", name), err))
			}
		}
	}
	return true
}

func (v *NomadNodeVisitor) getFuncDeclNameWithReceiver(t *ast.FuncDecl) string {
	name := t.Name.Name

	if t.Recv != nil {
		var recv string
		if stex, ok := t.Recv.List[0].Type.(*ast.StarExpr); ok {
			recv = stex.X.(*ast.Ident).Name
		} else if id, ok := t.Recv.List[0].Type.(*ast.Ident); ok {
			recv = id.Name
		}

		name = fmt.Sprintf("%s.%s.%s", v.activePackage.Name, recv, name)
	} else {
		name = fmt.Sprintf("%s.%s", v.activePackage.Name, name)
	}
	return name
}

func (v *NomadNodeVisitor) getTypesFuncNameWithReceiver(handler *types.Func) string {
	name := handler.Name()
	signature := handler.Type().(*types.Signature)
	if recv := signature.Recv(); recv != nil {
		recvSegments := strings.Split(recv.Type().String(), "/")
		recvName := recvSegments[len(recvSegments)-1]
		runes := []rune(recvName)
		recvName = string(runes[strings.Index(recvName, ".")+1:])
		name = fmt.Sprintf("%s.%s.%s", v.activePackage.Name, recvName, name)
	} else {
		name = fmt.Sprintf("%s.%s", v.activePackage.Name, name)
	}
	return name
}

func (v *NomadNodeVisitor) loadStructs(pkg *packages.Package) error {
	var structs map[string]*ast.TypeSpec
	var err error
	if structs, err = v.analyzer.GetStructs(pkg); err != nil {
		return err
	}

	for _, s := range structs {
		key := fmt.Sprintf("%s.%s", pkg.Name, s.Name)
		if _, ok := v.Structs[key]; ok {
			return fmt.Errorf("NomadNodeVisitor.loadStructs found duplicate key: %s", key)
		}
		v.Structs[key] = s
	}

	return nil
}
