package main

import (
	"go/ast"
)

// TypeSpecNode is used to create a tree of typespecs and track if they
// implement (or need to implement) the Copy method.
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

func (g *Generator) analyze() error {

	// need to generate the graph so that we know if *any* types are
	// Copy/Equal *before* we create the TargetTypes
	for _, file := range g.files {
		ast.Inspect(file, g.makeGraph)
		ast.Inspect(file, g.analyzeDecl)
	}

	for _, file := range g.files {
		for _, node := range file.Decls {
			switch node.(type) {

			case *ast.GenDecl:
				genDecl := node.(*ast.GenDecl)
				for _, spec := range genDecl.Specs {
					switch spec.(type) {
					case *ast.TypeSpec:
						typeSpec := spec.(*ast.TypeSpec)

						switch typeSpec.Type.(type) {
						case *ast.StructType:
							if g.isTarget(typeSpec.Name.Name) {
								t := &TargetType{Name: typeSpec.Name.Name, g: g}
								g.Targets = append(g.Targets, t)
								ast.Inspect(file, t.visitFields)
							}
						}
					}
				}
			}
		}
	}

	for _, typeName := range g.typeNames {
		// we already know these types need Copy to be copied, because the
		// user asked us to generate their Copy methods!
		g.typeSpecs[typeName].setIsCopier()
	}
	return nil
}

func (g *Generator) makeGraph(node ast.Node) bool {
	switch t := node.(type) {
	case *ast.TypeSpec:
		expr, ok := t.Type.(*ast.StructType)
		if !ok {
			return true
		}
		var ts *TypeSpecNode
		typeName := t.Name.Name
		ts, ok = g.typeSpecs[typeName]
		if !ok {
			ts = &TypeSpecNode{
				name:    typeName,
				fields:  map[string]*TypeSpecNode{},
				parents: map[string]*TypeSpecNode{},
			}
			g.typeSpecs[typeName] = ts
		}
		for _, field := range expr.Fields.List {
			switch expr := field.Type.(type) {
			case *ast.StarExpr:
				ident, ok := expr.X.(*ast.Ident)
				if ok {
					fieldTs, ok := g.typeSpecs[ident.Name]
					if !ok {
						fieldTs = &TypeSpecNode{
							name:    ident.Name,
							fields:  map[string]*TypeSpecNode{},
							parents: map[string]*TypeSpecNode{},
						}
					}
					ts.fields[ident.Name] = fieldTs
					fieldTs.parents[typeName] = ts
					g.typeSpecs[ident.Name] = fieldTs
				}
			}
		}

	}
	return true
}

func (g *Generator) analyzeDecl(node ast.Node) bool {
	switch t := node.(type) {
	case *ast.TypeSpec:
		g.needsCopyMethod(t)
	case *ast.FuncDecl:
		// if we find a Copy method, cache it as one we've seen
		if t.Recv != nil && t.Name.Name == "Copy" {
			var methodRecv string
			if stex, ok := t.Recv.List[0].Type.(*ast.StarExpr); ok {
				methodRecv = stex.X.(*ast.Ident).Name
			} else if id, ok := t.Recv.List[0].Type.(*ast.Ident); ok {
				methodRecv = id.Name
			}
			ts, ok := g.typeSpecs[methodRecv]
			if ok {
				ts.setIsCopier()
			}
		}
	}
	return true
}

func (g *Generator) needsCopyMethod(t *ast.TypeSpec) bool {
	name := t.Name.Name

	ts, ok := g.typeSpecs[name]
	if !ok {
		return false // ignore interfaces TODO?
	}

	// check if this has been set by one of its children previously
	if ts.isCopier() {
		return true
	}
	for _, field := range ts.fields {
		if field.isCopier() {
			ts.setIsCopier()
			return true
		}
	}

	expr, ok := t.Type.(*ast.StructType)
	if !ok {
		return false
	}
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
