package main

import (
	"fmt"
	"go/ast"
	"go/doc"
	"regexp"
	"sort"
	"strings"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-set/v2"
)

type Result struct {
	Package string
	Method  string
	Targets []*TypeSpecNode
}

// Analyzer holds the state of everything we see while analyzing the AST.  The
// go/ast package exposes a "visitor" interface so we can't simply return
// objects from free functions.
type Analyzer struct {
	log hclog.Logger

	// name of methods -> top-level desired targets
	methodsOfInterest map[string][]string

	// graph is the set of all TypeSpecNodes we've derived from the AST, keyed
	// by their type name. Each node has pointers to all its parent and child
	// types and well as the set of methods we need to implement for it
	graph map[string]*TypeSpecNode

	// unattachedMethods is a map of type name -> method name that's used when
	// we find a method of interest in the AST but haven't yet seen the
	// associated TypeSpec to add it to the graph. We can't guarantee that
	// TypeSpec declarations happen before the type's methods, so allows us to
	// make a single pass through the AST
	unattachedMethods map[string]*set.Set[string]
}

func NewAnalyzer(log hclog.Logger) *Analyzer {
	return &Analyzer{
		log:               log,
		methodsOfInterest: map[string][]string{},
		graph:             map[string]*TypeSpecNode{},
		unattachedMethods: map[string]*set.Set[string]{},
	}
}

// The analyze step returns a map of method names to the TargetTypes we'll
// generate for each of those methods.
func analyze(l hclog.Logger, pkg *ParsedPackage) ([]*Result, error) {

	a := NewAnalyzer(l)

	// First we need to figure out which "root" types we're interested in, and
	// which methods we'll generate
	err := a.findTargets(pkg)
	if err != nil {
		return nil, err
	}

	// Next we walk the AST from the package files, and build up a tree of
	// TypeSpecNodes and their implemented methods of interest.
	for _, file := range pkg.Files {
		ast.Inspect(file, a.makeGraphVisitor(file))
	}

	// Finally we trace the graph for each method to return a Result
	return a.collectTargets(pkg.Name), nil
}

const nomadGenerateRegex = `(?m)^nomad-generate: ([A-Za-z,]+)$`

// findTargets parses the package to look for docstrings that have a
// nomad-derive comment.
func (a *Analyzer) findTargets(pkg *ParsedPackage) error {

	// The go/ast package doesn't include docstrings on types unless we use an
	// weird declaration style (see https://github.com/golang/go/issues/27477).
	// use godoc's parser to find the docstrings with our target types and
	// methods.
	docPkg, err := doc.NewFromFiles(pkg.FileSet, pkg.Files, pkg.Dir)
	if err != nil {
		return err
	}

	re := regexp.MustCompile(nomadGenerateRegex)
	for _, t := range docPkg.Types {
		matches := re.FindAllStringSubmatch(t.Doc, -1)
		if len(matches) > 0 {
			for _, match := range matches {
				matchedMethods := strings.Split(match[1], ",")
				a.log.Debug("adding top-level target", "type", t.Name, "methods", matchedMethods)
				for _, method := range matchedMethods {
					if _, ok := a.methodsOfInterest[method]; !ok {
						a.methodsOfInterest[method] = []string{t.Name}
					} else {
						a.methodsOfInterest[method] = append(a.methodsOfInterest[method], t.Name)
					}

				}
			}
		}
	}

	if len(a.methodsOfInterest) == 0 {
		return fmt.Errorf("no top-level targets found")
	}

	return nil
}

// makeGraphVisitor returns a visitor function used by ast.Inspect to create a
// tree of TypeSpecNodes and their implemented methods of interest. Always
// returns true to continue to the next node.
func (a *Analyzer) makeGraphVisitor(file *ast.File) func(ast.Node) bool {
	return func(node ast.Node) bool {
		switch t := node.(type) {
		case *ast.TypeSpec:
			a.handleTypeSpecDecl(t)
		case *ast.FuncDecl:
			a.handleFuncDecl(t)
		}
		return true
	}
}

// handleTypeSpecDecl registers a TypeSpecNode for each TypeSpec we see, as well
// as one for each child field
func (a *Analyzer) handleTypeSpecDecl(t *ast.TypeSpec) {
	expr, ok := t.Type.(*ast.StructType)
	if !ok {
		// the type declaration isn't for a struct (ex. it's an interface or
		// function type), so there's nothing to do for this node.
		return
	}
	typeName := t.Name.Name
	ts := a.getOrCreateTypeSpec(typeName)

	// Check each of the types fields to see if it is a pointer to a type.
	for _, field := range expr.Fields.List {
		a.handleFieldDecl(ts, field)
	}
}

func (a *Analyzer) handleFuncDecl(t *ast.FuncDecl) {
	if t.Recv == nil {
		return // free function, not a method
	}
	methodName := t.Name.Name
	if _, ok := a.methodsOfInterest[methodName]; !ok {
		return // not a method we care about
	}

	// extract the name of the type of the receiver for this Copy method
	var methodRecv string
	if stex, ok := t.Recv.List[0].Type.(*ast.StarExpr); ok {
		methodRecv = stex.X.(*ast.Ident).Name
	} else if id, ok := t.Recv.List[0].Type.(*ast.Ident); ok {
		methodRecv = id.Name
	}
	// if the method receiver type is registered in the graph, mark it as
	// implementing this method
	ts, ok := a.graph[methodRecv]
	if ok {
		ts.implementsMethods.Insert(methodName)
		return
	}

	// ... otherwise, cache that we've seen this method so we can attach it
	// later
	if _, ok := a.unattachedMethods[methodRecv]; !ok {
		a.unattachedMethods[methodRecv] = set.From[string]([]string{methodName})
	} else {
		a.unattachedMethods[methodRecv].Insert(methodName)
	}
}

func (a *Analyzer) handleFieldDecl(ts *TypeSpecNode, field *ast.Field) *TargetField {
	f := &TargetField{
		Name:       field.Names[0].Name,
		Field:      field,
		hasMethods: set.New[string](0),
	}
	switch t := field.Type.(type) {
	case *ast.Ident:
		f.TypeName = field.Type.(*ast.Ident).Name
		f.Kind = f.TypeName
		// For direct struct references (not pointers) the type
		// Name will be returned so we correct it here.
		if !f.IsPrimitive() {
			f.Kind = "struct"
		}
	case *ast.ArrayType: // note: this includes either array or slice
		f.Kind = "array"
		f.evaluateArrayOrMapValueType(t)
		if f.ValueType.Kind == "pointer" {
			// check to see if field's type been registered already
			fts := a.getOrCreateTypeSpec(f.ValueType.pointerIdent)

			// record the parent-child relationships between the current
			// type and the field type
			ts.children.Insert(fts.Name)
			fts.parents.Insert(ts.Name)
		}

	case *ast.MapType:
		f.Kind = "map"
		f.evaluateArrayOrMapValueType(t)
		f.KeyType = &TargetField{
			TypeName:   t.Key.(*ast.Ident).Name,
			hasMethods: set.New[string](0),
		}
		if f.ValueType.Kind == "pointer" {
			// check to see if field's type been registered already
			fts := a.getOrCreateTypeSpec(f.ValueType.pointerIdent)

			// record the parent-child relationships between the current
			// type and the field type
			ts.children.Insert(fts.Name)
			fts.parents.Insert(ts.Name)
		}
	case *ast.StructType:
		f.Kind = "struct"
		// TODO: where can we get the Ident from?
		//ts := f.g.typeSpecs[ident]
		//f.isCopier = ts != nil && ts.isCopier()

	case *ast.StarExpr:
		f.Kind = "pointer"
		ident, ok := t.X.(*ast.Ident)
		if ok {
			// check to see if field's type been registered already
			fts := a.getOrCreateTypeSpec(ident.Name)

			// record the parent-child relationships between the current
			// type and the field type
			ts.children.Insert(fts.Name)
			fts.parents.Insert(ts.Name)

		}
	}
	ts.Fields = append(ts.Fields, f)

	return f

}

// getOrCreateTypeSpec gets a TypeSpecNode by name or creates a new one if not
// found, and registers it in the Analyzer's graph.
func (a *Analyzer) getOrCreateTypeSpec(typeName string) *TypeSpecNode {
	// Check to see if it's been registered already
	ts, ok := a.graph[typeName]
	if ok {
		return ts
	}

	// if not create a new TypeSpecNode
	ts = &TypeSpecNode{
		Name:              typeName,
		children:          set.New[string](0),
		parents:           set.New[string](0),
		needsMethods:      set.New[string](0),
		implementsMethods: set.New[string](0),
	}

	// if we've previously seen methods of interest for this type, set them on
	// the node and clear them from the analyzer's cache
	if methods, ok := a.unattachedMethods[typeName]; ok {
		ts.implementsMethods = methods
		delete(a.unattachedMethods, typeName)
	}

	// register the instance
	a.log.Debug("registering TypeSpec", "type", typeName)
	a.graph[typeName] = ts
	return ts
}

// collectTargets recursively walks the graph starting from each of the top
// target types for each method they need
func (a *Analyzer) collectTargets(pkgName string) []*Result {

	results := []*Result{}

	for method, types := range a.methodsOfInterest {
		result := &Result{
			Package: pkgName,
			Method:  method,
			Targets: []*TypeSpecNode{},
		}
		for _, typeName := range types {
			a.collectType(typeName, method, result)
			results = append(results, result)
		}

		sort.Slice(result.Targets, func(i, j int) bool {
			return result.Targets[i].Name < result.Targets[j].Name
		})
	}

	return results
}

func (a *Analyzer) collectType(typeName string, method string, result *Result) {
	a.log.Debug("collecting result", "type", typeName, "method", method)
	t := a.graph[typeName]
	if !t.implementsMethods.Contains(method) {
		t.needsMethods.Insert(method)
		result.Targets = append(result.Targets, t)
	}
	for _, field := range t.Fields {
		field.setHasMethod(method)
	}
	for _, child := range t.children.Slice() {
		a.collectType(child, method, result)
	}
}
