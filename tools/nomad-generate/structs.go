package main

import (
	"fmt"
	"go/ast"
	"strings"
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

// TargetType represents a type that has been targeted for generation by passing
// the -type option to the go;generate command.
type TargetType struct {
	Name           string         // Name of the type we're generating methods for
	methods        []string       // Methods we want to generate - valid values are copy, equals, and all. Will eventually support diff and merge as well.
	excludedFields []string       // Fields to exclude from generated method
	Fields         []*TargetField // State bad filled by the the visitFields method
	g              *Generator     // Reference back to generator root
}

// Abbr returns the first letter of the type name and is used by the template
// to for rendering receiver function signatures.
func (t *TargetType) Abbr() string {
	return strings.ToLower(string(t.Name[0]))
}

// hasMethod is a template method used by the IsCopy etc methods
func (t *TargetType) hasMethod(methodName string) bool {
	for _, method := range t.Methods() {
		if strings.ToLower(method) == "all" {
			return true
		}
		if strings.ToLower(methodName) == strings.ToLower(method) {
			return true
		}
	}
	return false
}

// IsCopy is used by the template to determine whether to render a Copy method
// for the Target.
func (t *TargetType) IsCopy() bool {
	return t.hasMethod("copy")
}

// IsEquals is used by the template to determine whether to render an Equals method
// for the Target.
func (t *TargetType) IsEquals() bool {
	return t.hasMethod("equals")
}

// IsDiff is used by the template to determine whether to render a Diff method
// for the Target.
func (t *TargetType) IsDiff() bool {
	return t.hasMethod("diff")
}

// IsMerge is used by the template to determine whether to render a Merge method
// for the Target.
func (t *TargetType) IsMerge() bool {
	return t.hasMethod("merge")
}

// Methods lazy loads the members slice by evaluating the methods from the
// generator, and extracting the ones that map to the Target.
// TODO: refactor to a single type per generator call.
func (t *TargetType) Methods() []string {
	if t.methods == nil {
		var m []string
		for _, method := range t.g.methods {
			if strings.Contains(method, t.Name) {
				md := strings.TrimPrefix(method, fmt.Sprintf("%s.", t.Name))
				m = append(m, md)
			}
		}

		if len(m) > 0 {
			t.methods = m
		} else {
			t.methods = make([]string, 0)
		}

	}
	return t.methods
}

// ExcludedFields lazy loads the excludedFields slice by evaluating the excludedMethods
// from the generator, and extracting the ones that map to the Target.
// TODO: refactor to a single type per generator call.
func (t *TargetType) ExcludedFields() []string {
	if t.excludedFields == nil {
		var e []string
		for _, excludedField := range t.g.excludedFields {
			if strings.Index(excludedField, t.Name) > -1 {
				e = append(e, strings.TrimPrefix(excludedField, fmt.Sprintf("%s.", t.Name)))
			}
		}

		if len(e) > 0 {
			t.excludedFields = e
		} else {
			t.excludedFields = make([]string, 0)
		}

	}
	return t.excludedFields
}

// visitFields is invoked by ast.Inspect which is the primary Visitor pattern root.
// It is responsible for building the Fields slice for the Target. It also invokes
// ast.Inspect on each field and passes resolveTypeAndKind, which handles field
// level type introspection.
func (t *TargetType) visitFields(node ast.Node) bool {
	switch node.(type) {
	case *ast.TypeSpec:
		typeSpec := node.(*ast.TypeSpec)
		if typeSpec.Name.Name == t.Name {
			expr := typeSpec.Type.(*ast.StructType)
			for _, field := range expr.Fields.List {
				if t.fieldIsExcluded(field.Names[0].Name) {
					continue
				}

				targetField := &TargetField{Name: field.Names[0].Name, Field: field, g: t.g}
				t.Fields = append(t.Fields, targetField)
				ast.Inspect(field, targetField.resolveTypeAndKind)
			}
		}
	}
	return true
}

// fieldIsExcluded evaluates whether a field has been marked for exclusion.
func (t *TargetType) fieldIsExcluded(name string) bool {
	for _, exclude := range t.ExcludedFields() {
		if exclude == name {
			return true
		}
	}
	return false
}

// TargetField maintains state about individual TargetType fields, and provides
// field level introspection and metadata used by the template to rendering
// the appropriate syntax for a given field.
type TargetField struct {
	Kind     string     // Used to determine groups of types that can be treated similarly. Valid values are struct, array, map, pointer, or any primitive type
	Name     string     // Name of the field
	Field    *ast.Field // The field ast Node
	TypeName string     // TypeName of the field

	KeyType   *TargetField // the type of a map key
	ValueType *TargetField // the type of a map or array value

	isCopier bool       // Indicates whether this type implement Copy
	g        *Generator // Reference to generator root
}

// IsPrimitive is used to functionally evaluate if a field is of a kind other than primitive.
// TODO:
func (f *TargetField) IsPrimitive() bool {
	return !(f.IsPointer() || f.IsStruct() || f.IsArray() || f.IsMap())
}

func (f *TargetField) IsArray() bool {
	return f.Kind == "array"
}

func (f *TargetField) IsStruct() bool {
	return f.Kind == "struct"
}

func (f *TargetField) IsPointer() bool {
	return f.Kind == "pointer"
}

func (f *TargetField) IsMap() bool {
	return f.Kind == "map"
}

func (f *TargetField) IsCopier() bool {
	return f.isCopier
}

func (f *TargetField) resolveTypeAndKind(node ast.Node) bool {
	if len(f.TypeName) < 1 {
		switch node.(type) {
		case *ast.Field:
			if node.(*ast.Field).Names[0].Name == f.Name {
				switch t := node.(*ast.Field).Type.(type) {
				case *ast.Ident:
					f.TypeName = node.(*ast.Field).Type.(*ast.Ident).Name
					f.Kind = f.TypeName
					// For direct struct references (not pointers) the type
					// Name will be returned so we correct it here.
					if !f.IsPrimitive() {
						f.Kind = "struct"
					}
				case *ast.ArrayType:
					f.Kind = "array"
					f.evaluateValueType(t)
				case *ast.MapType:
					f.Kind = "map"
					f.evaluateValueType(t)
					f.KeyType = &TargetField{
						TypeName: t.Key.(*ast.Ident).Name,
						g:        f.g,
					}

				case *ast.StructType:
					f.Kind = "struct"
					// TODO: where can we get the Ident from?
					//ts := f.g.typeSpecs[ident]
					//f.isCopier = ts != nil && ts.isCopier()

				case *ast.StarExpr:
					f.Kind = "pointer"
					ident := t.X.(*ast.Ident).Name
					ts := f.g.typeSpecs[ident]
					f.isCopier = ts != nil && ts.isCopier()
				}
			}
		default:
			f.TypeName = fmt.Sprintf("%+v", node)
		}
	}
	return true
}

// evaluateValueType resolves the underlying type and kind of ArrayType and MapType
// fields so that the templates can render the proper syntax for the underlying
// elements.
func (f *TargetField) evaluateValueType(expr ast.Expr) {
	var valueTypeName string
	var ident string
	var kind string

	switch t := expr.(type) {
	case *ast.ArrayType:
		expr, ok := t.Elt.(*ast.StarExpr)
		if ok {
			ident = expr.X.(*ast.Ident).Name
			valueTypeName = "*" + ident
			kind = "pointer"
		} else {
			ident = t.Elt.(*ast.Ident).Name
			valueTypeName = ident
			kind = ident
		}
	case *ast.MapType:
		expr, ok := t.Value.(*ast.StarExpr)
		if ok {
			ident = expr.X.(*ast.Ident).Name
			valueTypeName = "*" + ident
			kind = "pointer"
		} else {
			ident = t.Value.(*ast.Ident).Name
			valueTypeName = ident
			kind = ident
		}
	}

	ts := f.g.typeSpecs[ident]

	f.ValueType = &TargetField{
		Kind:     kind,
		TypeName: valueTypeName,
		isCopier: ts != nil && ts.isCopier(),
		g:        f.g,
	}
}
