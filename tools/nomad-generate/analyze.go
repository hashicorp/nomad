package main

import (
	"go/ast"
)

// TypeSpecNode is used to create a tree of TypeSpecs and track if they
// implement (or need to implement) the Copy method. This type is populated
// during the analysis phase, and is necessary because the types we are targeting
// may have member fields that we are not targeting, but we will still need
// information about, such as whether the type of the member field implements
// copy itself.
type TypeSpecNode struct {
	name           string
	fields         map[string]*TypeSpecNode
	parents        map[string]*TypeSpecNode
	implementsCopy bool
}

// setIsCopier sets this type as Copy and all of its parents as well
func (t *TypeSpecNode) setIsCopier() {
	t.implementsCopy = true
	for _, p := range t.parents {
		p.implementsCopy = true
	}
}

func (t *TypeSpecNode) isCopier() bool {
	if t == nil {
		return false
	}
	return t.implementsCopy
}

// analyze processes the full graph to know which types are already Copy/Equal
// and then evaluates the targets.
func (g *Generator) analyze() error {
	// need to generate the graph so that we know if *any* types are
	// Copy/Equal *before* we create the TargetTypes
	for _, file := range g.files {
		ast.Inspect(file, g.makeGraph)
		ast.Inspect(file, g.analyzeDecl)
	}

	// TODO: Do we still need this?
	for _, typeName := range g.typeNames {
		// we already know these types need Copy to be copied, because the
		// user asked us to generate their Copy methods!
		g.typeSpecs[typeName].setIsCopier()
	}

	// Now that we've analyzed the full graph, we can build the Targets
	for _, file := range g.files {
		for _, node := range file.Decls {
			switch node.(type) {
			case *ast.GenDecl:
				g.evaluateTarget(node, file)
			}
		}
	}

	// TODO: Is there err handling we can do or should we change signature.
	return nil
}

// makeGraph is a visitor function used by ast.Inspect to find all struct
// types within a tree node and its children, and then register a TypeSpecNode
// instance with the generator.
func (g *Generator) makeGraph(node ast.Node) bool {
	switch t := node.(type) {
	case *ast.TypeSpec:
		expr, ok := t.Type.(*ast.StructType)
		// if it's not a struct exit
		if !ok {
			return true
		}
		var ts *TypeSpecNode
		typeName := t.Name.Name
		ts = g.getOrCreateTypeSpec(typeName)
		// Check each of the types fields to see if it is a pointer to a type.
		for _, field := range expr.Fields.List {
			switch expr := field.Type.(type) {
			case *ast.StarExpr:
				// If it is a pointer, figure out the underlying type.
				ident, ok := expr.X.(*ast.Ident)
				if ok {
					// check to see if it's been registered already
					fieldTypeSpec := g.getOrCreateTypeSpec(ident.Name)
					ts.fields[ident.Name] = fieldTypeSpec
					fieldTypeSpec.parents[typeName] = ts
				}
			}
		}

	}
	return true
}

// getOrCreateTypSpec gets a TypeSpecNode by name or creates a new one if not found.
func (g *Generator) getOrCreateTypeSpec(typeName string) *TypeSpecNode {
	var ts *TypeSpecNode
	// Check to see if it's been registered already
	ts, ok := g.typeSpecs[typeName]
	// if not create a new TypeSpecNode
	if !ok {
		ts = &TypeSpecNode{
			name:    typeName,
			fields:  map[string]*TypeSpecNode{},
			parents: map[string]*TypeSpecNode{},
		}
		// register the instance
		g.typeSpecs[typeName] = ts
	}
	return ts
}

// analyzeDecl visits a tree node and ensures that graph members that need a copy
// method are appropriately marked.
func (g *Generator) analyzeDecl(node ast.Node) bool {
	switch t := node.(type) {
	case *ast.TypeSpec:
		g.needsCopyMethod(t)
	case *ast.FuncDecl:
		// if we find a Copy method, cache it as one we've seen
		if t.Recv != nil && t.Name.Name == "Copy" {
			// extract the name of the type of the receiver for this Copy method
			var methodRecv string
			if stex, ok := t.Recv.List[0].Type.(*ast.StarExpr); ok {
				methodRecv = stex.X.(*ast.Ident).Name
			} else if id, ok := t.Recv.List[0].Type.(*ast.Ident); ok {
				methodRecv = id.Name
			}
			// Check if registered, and set isCopier if found
			ts, ok := g.typeSpecs[methodRecv]
			if ok {
				ts.setIsCopier()
			}
		}
	}
	return true
}

// needsCopyMethod evaluates whether a type needs a copy method.
func (g *Generator) needsCopyMethod(t *ast.TypeSpec) bool {
	name := t.Name.Name

	ts, ok := g.typeSpecs[name]
	// if not registered return
	if !ok {
		return false // ignore interfaces TODO?
	}

	// Return true if this has been set by one of its children previously.
	if ts.isCopier() {
		return true
	}

	// If any child is a copier, set and return true
	for _, field := range ts.fields {
		if field.isCopier() {
			ts.setIsCopier()
			return true
		}
	}

	// if type is not a struct return false
	expr, ok := t.Type.(*ast.StructType)
	if !ok {
		return false
	}

	// if any of its fields are pointers, maps, or array types set and return true.
	for _, field := range expr.Fields.List {
		switch field.Type.(type) {
		case *ast.StarExpr, *ast.MapType, *ast.ArrayType:
			ts.setIsCopier()
			return true
		default:
			// primitives are never Copier, and struct values will be marked
			// as Copier when we hit needsCopyMethod on that struct value's
			// type spec
		}
	}
	return false
}
