package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"golang.org/x/tools/go/packages"
	"strings"
)

type loggerFunc func(args ...interface{})

type NomadNodeVisitor struct {
	HandlerFuncs map[string]*HandlerFuncInfo
	Structs      map[string]*ast.TypeSpec
	packages     map[string]*packages.Package
	analyzer     *Analyzer
	logger       loggerFunc
	files        []*ast.File
	fileSets     []*token.FileSet
}

func (v *NomadNodeVisitor) SetPackages(pkgs []*packages.Package) error {
	if v.packages == nil {
		v.packages = make(map[string]*packages.Package)
	}

	if v.Structs == nil {
		v.Structs = make(map[string]*ast.TypeSpec)
	}

	if v.HandlerFuncs == nil {
		v.HandlerFuncs = make(map[string]*HandlerFuncInfo)
	}

	// Must load all structs from all packages BEFORE loading Handlers.
	for _, pkg := range pkgs {
		if err := v.loadStructs(pkg); err != nil {
			return err
		}
	}

	for _, pkg := range pkgs {
		if _, ok := v.packages[pkg.Name]; ok {
			return fmt.Errorf(fmt.Sprintf("NomadVisitor.SetPackages: Package %s alread exists", pkg.Name))
		}

		v.packages[pkg.Name] = pkg
		v.SetActiveFileSet(pkg.Fset)

		if err := v.loadHandlers(pkg); err != nil {
			return err
		}
	}
	return nil
}

func (v *NomadNodeVisitor) loadHandlers(pkg *packages.Package) error {
	handlers := v.analyzer.GetHttpHandlers(pkg)
	for key, handler := range handlers {
		if _, ok := v.HandlerFuncs[v.getTypesFuncNameWithReceiver(handler)]; ok {
			return fmt.Errorf("NomadVisitor.loadHandlers package %s alread exists", key)
		}

		v.HandlerFuncs[v.getTypesFuncNameWithReceiver(handler)] = &HandlerFuncInfo{
			Func:     handler,
			Structs:  v.Structs,
			logger:   v.logger,
			analyzer: v.analyzer,
			fileSet:  v.GetActiveFileSet(),
		}
	}
	return nil
}

func (v *NomadNodeVisitor) DebugPrint() {
	for key, fn := range v.HandlerFuncs {
		v.logger(fmt.Sprintf("%s: Response Type: %s\n - Params/Source: %s\n", key, fn.ResponseType.Name, fn.Source))
	}
}

func (v *NomadNodeVisitor) Files() []*ast.File {
	return v.files
}

func (v *NomadNodeVisitor) FileSets() []*token.FileSet {
	return v.fileSets
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
			v.logger(fmt.Errorf("VisitNode.getSourceL %v\n", err))
			return true
		}
		name := v.getFuncDeclNameWithReceiver(t)
		// If not a handler then don't add the func
		if _, ok := v.HandlerFuncs[name]; !ok {
			return true
		}

		// v.logger(fmt.Sprintf("Found HandlerFuncInfo for %s", name))

		params := ""
		for _, param := range t.Type.Params.List {
			params = fmt.Sprintf("%s|%s ", param.Names[0].Name, param.Type)
		}
		src = fmt.Sprintf("%s - %s", params, src)

		if _, ok := v.HandlerFuncs[name]; !ok {
			panic(fmt.Sprintf(fmt.Sprintf("VisitNode failed to resolve HandlerFuncInfo for %s", name)))
		} else {
			funcInfo := v.HandlerFuncs[name]
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

		name = fmt.Sprintf("%s.%s", recv, name)
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
		name = fmt.Sprintf("%s.%s", recvName, name)
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
