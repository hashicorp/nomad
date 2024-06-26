package main

import (
	"go/ast"
	"strings"

	"github.com/hashicorp/go-set/v2"
)

// TypeSpecNode is are the nodes of a graph derived from the TypeSpecs we find
// in the ast. It tracks the parent and child types (by field) and whether the
// type implements (or needs to implement) the desired methods.
type TypeSpecNode struct {
	Name string

	// Fields is a slice of field metadata filed when we visit the TypeSpec and
	// used for rendering the TypeSpecNode
	Fields []*TargetField

	// spec is the underlying AST node for the type
	spec *ast.TypeSpec

	// children is the set of types used in this type's non-primitive fields,
	// keyed by TypeSpec name
	children *set.Set[string]

	// parents is the set of types for whom this type is used in a non-primitive
	// field, keyed by TypeSpec name
	parents *set.Set[string]

	// implementsMethods is the set of methods we would need to implement for
	// this type, but already exist in the AST. Methods discovered later and
	// added here should be removed from .needsMethods
	implementsMethods *set.Set[string]

	// needsMethods is the set of methods we need to implement for this type
	// that don't already exist in the AST
	needsMethods *set.Set[string]
}

// Abbr returns the first letter of the type name and is used by the template
// to for rendering receiver function signatures.
func (t *TypeSpecNode) Abbr() string {
	return strings.ToLower(string(t.Name[0]))
}

// hasMethod is a template method used by the IsCopy etc methods
func (t *TypeSpecNode) hasMethod(methodName string) bool {
	return t.needsMethods.Contains(methodName) ||
		t.implementsMethods.Contains(methodName)
}

// IsCopy is used by the template to determine whether to render a Copy method for the Target.
func (t *TypeSpecNode) IsCopy() bool {
	return t.hasMethod("Copy")
}

// IsEquals is used by the template to determine whether to render an Equals method for the Target.
func (t *TypeSpecNode) IsEquals() bool {
	return t.hasMethod("Equals")
}

// IsDiff is used by the template to determine whether to render a Diff method for the Target.
func (t *TypeSpecNode) IsDiff() bool {
	return t.hasMethod("Diff")
}

// IsMerge is used by the template to determine whether to render a Merge method for the Target.
func (t *TypeSpecNode) IsMerge() bool {
	return t.hasMethod("Merge")
}

// Methods lazy loads the members slice by evaluating the methods from the
// generator, and extracting the ones that map to the Target.
// TODO: refactor to a single type per generator call.
func (t *TypeSpecNode) Methods() []string {
	return t.implementsMethods.Slice()
}

// TargetField maintains state about individual TargetType fields, and provides
// field level introspection and metadata used by the template to rendering
// the appropriate syntax for a given field.
type TargetField struct {
	Kind         string           // Used to determine groups of types that can be treated similarly. Valid values are struct, array, map, pointer, or any primitive type
	Name         string           // Name of the field
	Field        *ast.Field       // The field ast Node
	TypeName     string           // TypeName of the field
	KeyType      *TargetField     // the type of a map key
	ValueType    *TargetField     // the type of a map or array value
	pointerIdent string           // The TypeName "dereferenced" (without '*')
	hasMethods   *set.Set[string] // A set of methods the type of this field implements
}

// IsPrimitive functionally evaluates if a field is of a kind other than primitive.
// Used during template rendering.
func (f *TargetField) IsPrimitive() bool {
	return !(f.IsPointer() || f.IsStruct() || f.IsArray() || f.IsMap())
}

// IsArray evaluates if a field is of kind "array". Used during template rendering.
func (f *TargetField) IsArray() bool {
	return f.Kind == "array"
}

// IsStruct evaluates if a field is of kind "struct". Used during template rendering.
func (f *TargetField) IsStruct() bool {
	return f.Kind == "struct"
}

// IsPointer evaluates if a field is of kind "pointer". Used during template rendering.
func (f *TargetField) IsPointer() bool {
	return f.Kind == "pointer"
}

// IsMap evaluates if a field is of kind "map". Used during template rendering.
func (f *TargetField) IsMap() bool {
	return f.Kind == "map"
}

// IsCopy evaluates if a field's type implements Copy. Used during template rendering.
func (f *TargetField) IsCopy() bool {
	return f.hasMethods.Contains("Copy")
}

// IsDiff evaluates if a field's type implements Diff. Used during template rendering.
func (f *TargetField) IsDiff() bool {
	return f.hasMethods.Contains("Diff")
}

func (f *TargetField) setHasMethod(method string) {
	f.hasMethods.Insert(method)
	if f.ValueType != nil && f.ValueType.Kind == "pointer" {
		f.ValueType.hasMethods.Insert(method)
	}
}

// evaluateArrayOrMapValueType resolves the underlying type and kind of
// ArrayType and MapType fields so that the templates can render the proper
// syntax for the underlying elements.
func (f *TargetField) evaluateArrayOrMapValueType(expr ast.Expr) {
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

	f.ValueType = &TargetField{
		Kind:         kind,
		TypeName:     valueTypeName,
		pointerIdent: ident,
		hasMethods:   set.New[string](0),
	}
}
