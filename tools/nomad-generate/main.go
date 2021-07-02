package main

import (
	"bytes"
	"embed"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"os"
	"strings"
	"text/template"

	"golang.org/x/tools/go/packages"
)

//go:embed structs.copy.tmpl
var copyTmpl embed.FS

//go:embed structs.equals.tmpl
var equalsTmpl embed.FS

//go:embed structs.diff.tmpl
var diffTmpl embed.FS

//go:embed structs.merge.tmpl
var mergeTmpl embed.FS

type stringSliceFlag []string

func (s *stringSliceFlag) String() string {
	return fmt.Sprintf("%s", *s)
}

func (s *stringSliceFlag) Set(value string) error {
	*s = append(*s, value)
	return nil
}

// Generator holds the state of the analysis. Used as the data model and executor
// to render templates.
type Generator struct {
	packageDir     string
	files          []*ast.File
	Targets        []*TargetType
	typeSpecs      map[string]*TypeSpecNode
	typeNames      []string
	methods        []string
	excludedFields []string
}

func main() {
	var excludedFieldFlags stringSliceFlag
	var typeNameFlags stringSliceFlag
	var methodFlags stringSliceFlag
	var packageDir string

	flag.Var(&excludedFieldFlags, "exclude", "list of Fields to exclude from Copy")
	flag.Var(&typeNameFlags, "type", "types for which to generate Copy methodFlags")
	flag.Var(&methodFlags, "method", "methodFlags to generate - defaults to all")
	flag.StringVar(&packageDir, "packageDir", "./", "The source dir to target")
	flag.Parse()

	if len(typeNameFlags) == 0 {
		fmt.Println("at least one -type flag needed to generate Copy")
		os.Exit(2)
	}

	g := &Generator{
		packageDir:     packageDir,
		typeNames:      typeNameFlags,
		methods:        methodFlags,
		excludedFields: excludedFieldFlags,
		typeSpecs:      map[string]*TypeSpecNode{},
	}
	err := run(g)
	if err != nil {
		fmt.Println(err)
		os.Exit(2)
	}
}

func run(g *Generator) error {
	var err error
	var pkgs []*packages.Package

	if pkgs, err = g.loadPackages(); err != nil {
		return fmt.Errorf("error loading packages: %v", err)
	}

	if err = g.parsePackages(pkgs); err != nil {
		return fmt.Errorf("error parsing packages: %v", err)
	}

	if len(pkgs) == 0 {
		return fmt.Errorf("did not parse any packages")
	}

	if err = g.analyze(); err != nil {
		return fmt.Errorf("error analyzing: %v", err)
	}

	if len(g.typeSpecs) == 0 {
		return fmt.Errorf("did not analyze any types")
	}

	if err = g.generate(); err != nil {
		return fmt.Errorf("error generating: %v", err)
	}

	return nil
}

// loadPackages loads the source code for analysis
func (g *Generator) loadPackages() ([]*packages.Package, error) {
	// TODO: Determine which of these we really need
	const loadMode = packages.NeedName |
		packages.NeedFiles |
		packages.NeedCompiledGoFiles |
		packages.NeedImports |
		packages.NeedDeps |
		packages.NeedExportsFile |
		packages.NeedTypes |
		packages.NeedSyntax |
		packages.NeedTypesInfo |
		packages.NeedTypesSizes |
		packages.NeedModule

	cfg := &packages.Config{
		Dir:  g.packageDir,
		Mode: loadMode,
	}

	pkgs, err := packages.Load(cfg, ".")
	return pkgs, err
}

// parsePackages iterates over the package source and ensures each go file is processed.
func (g *Generator) parsePackages(pkgs []*packages.Package) error {
	for _, pkg := range pkgs {

		if len(pkg.Errors) > 0 {
			return pkg.Errors[0]
		}

		for _, goFile := range pkg.GoFiles {
			if err := g.parseGoFile(goFile); err != nil {
				return err
			}
		}
	}

	return nil
}

// parseGoFile parses an individual go file and adds it to the generator's files member
func (g *Generator) parseGoFile(goFile string) error {
	// Create the AST by parsing src.
	fileSet := token.NewFileSet() // positions are relative to fset
	file, err := parser.ParseFile(fileSet, goFile, nil, 0)
	if err != nil {
		return fmt.Errorf("could not parse file: %v\n", err)
	}

	g.files = append(g.files, file)
	return nil
}

// evaluateTarget traverses a generic declaration node to determine if it is a
// struct, and if so whether it is a struct we are targeting. If it is a targeted
// type, it then adds it to the Targets slice, and calls ast.Inspect in order to
// evaluate all of the type's fields.
func (g *Generator) evaluateTarget(node ast.Decl, file *ast.File) {
	genDecl := node.(*ast.GenDecl)
	for _, spec := range genDecl.Specs {
		switch spec.(type) {
		case *ast.TypeSpec:
			typeSpec := spec.(*ast.TypeSpec)
			switch typeSpec.Type.(type) {
			case *ast.StructType:
				if g.isTarget(typeSpec.Name.Name) {
					// If the struct is targeted, add a new Target and visit its fields
					t := &TargetType{Name: typeSpec.Name.Name, g: g}
					g.Targets = append(g.Targets, t)
					ast.Inspect(file, t.visitFields)
				}
			}
		}
	}
}

// isTarget checks to see if a type's name was passed as a -type argument to the generator
func (g *Generator) isTarget(name string) bool {
	for _, typeName := range g.typeNames {
		if name == typeName {
			return true
		}
	}
	return false
}

func (g *Generator) generate() error {
	var err error
	if err = g.render("copy"); err != nil {
		return errors.New(fmt.Sprintf("generate.copy: %v", err))
	}

	if err = g.render("equals"); err != nil {
		return errors.New(fmt.Sprintf("generate.equals: %v", err))
	}

	//if err = g.render("diff"); err != nil {
	//	return errors.New(fmt.Sprintf("generate.diff: %v", err))
	//}

	//if err = g.render("merge"); err != nil {
	//	return errors.New(fmt.Sprintf("generate.merge: %v", err))
	//}

	return nil
}

func (g *Generator) render(targetFunc string) error {
	var err error
	targetFileName := fmt.Sprintf("./structs.%s.go", targetFunc)

	var templateFile embed.FS

	switch targetFunc {
	case "copy":
		templateFile = copyTmpl
	case "equals":
		templateFile = equalsTmpl
	case "diff":
		templateFile = diffTmpl
	case "merge":
		templateFile = mergeTmpl
	}

	var buf bytes.Buffer
	err = g.execTemplate(&buf, templateFile)
	if err != nil {
		return err
	}

	formatted := g.format(buf.Bytes())

	err = os.WriteFile(targetFileName, formatted, 0644)
	if err != nil {
		return err
	}

	return nil
}

func (g *Generator) execTemplate(w io.Writer, file embed.FS) error {
	if len(g.Targets) < 1 {
		return errors.New("generate.render.execTemplate: no targets found")
	}
	tmpl, err := template.ParseFS(file, "*")
	if err != nil {
		return errors.New(fmt.Sprintf("generate.render.execTemplate: %v", err))
	}
	return tmpl.Execute(w, g)
}

func (g *Generator) format(buf []byte) []byte {
	src, err := format.Source(buf)
	if err != nil {
		fmt.Printf("invalid Go generated: %s\n", err) // should never happen
		return buf
	}
	return src
}

// TargetField maintains state about individual TargetType fields, and provides
// field level introspection and metadata used by the template to rendering
// the appropriate syntax for a given field.
type TargetField struct {
	Kind      string       // Used to determine groups of types that can be treated similarly. Valid values are struct, array, map, pointer, or any primitive type
	Name      string       // Name of the field
	Field     *ast.Field   // The field ast Node
	TypeName  string       // TypeName of the field
	KeyType   *TargetField // the type of a map key
	ValueType *TargetField // the type of a map or array value
	isCopier  bool         // Indicates whether this type implement Copy
	g         *Generator   // Reference to generator root
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

// IsCopier evaluates if a field's type implements copy. Used during template rendering.
func (f *TargetField) IsCopier() bool {
	return f.isCopier
}

// evaluateFieldTypeAndKind inspects a node to determine if it a field, and if
// so it performs the necessary type and kind inspection on the field.
func (f *TargetField) evaluateFieldTypeAndKind(node ast.Node) bool {
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
					f.evaluateArrayOrMapValueType(t)
				case *ast.MapType:
					f.Kind = "map"
					f.evaluateArrayOrMapValueType(t)
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
					ts := f.g.typeSpecs[ident] // should be safe since graph is built
					f.isCopier = ts != nil && ts.isCopier()
				}
			}
		default:
			f.TypeName = fmt.Sprintf("%+v", node)
		}
	}
	return true
}

// evaluateArrayOrMapValueType resolves the underlying type and kind of ArrayType and MapType
// fields so that the templates can render the proper syntax for the underlying
// elements.
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

	ts := f.g.typeSpecs[ident]

	f.ValueType = &TargetField{
		Kind:     kind,
		TypeName: valueTypeName,
		isCopier: ts != nil && ts.isCopier(),
		g:        f.g,
	}
}

// TargetType represents a type that has been targeted for generation by passing
// the -type option to the go;generate command.
type TargetType struct {
	Name           string         // Name of the type we're generating methods for
	methods        []string       // Methods we want to generate - valid values are copy, equals, and all. Will eventually support diff and merge.
	excludedFields []string       // Fields to exclude from generated methods
	Fields         []*TargetField // Field metadata state bag filled by the visitFields method
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

// IsCopy is used by the template to determine whether to render a Copy method for the Target.
func (t *TargetType) IsCopy() bool {
	return t.hasMethod("copy")
}

// IsEquals is used by the template to determine whether to render an Equals method for the Target.
func (t *TargetType) IsEquals() bool {
	return t.hasMethod("equals")
}

// IsDiff is used by the template to determine whether to render a Diff method for the Target.
func (t *TargetType) IsDiff() bool {
	return t.hasMethod("diff")
}

// IsMerge is used by the template to determine whether to render a Merge method for the Target.
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
// ast.Inspect on each field and passes evaluateFieldTypeAndKind, which handles field
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
				ast.Inspect(field, targetField.evaluateFieldTypeAndKind)
			}
		}
	}
	return true
}

func (t *TargetType) fieldIsExcluded(name string) bool {
	for _, exclude := range t.ExcludedFields() {
		if exclude == name {
			return true
		}
	}
	return false
}
