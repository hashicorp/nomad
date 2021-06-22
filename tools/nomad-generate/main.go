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

func (g *Generator) loadPackages() ([]*packages.Package, error) {
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

func (g *Generator) parsePackages(pkgs []*packages.Package) error {
	for _, pkg := range pkgs {

		if len(pkg.Errors) > 0 {
			return pkg.Errors[0]
		}

		for _, goFile := range pkg.GoFiles {
			// Create the AST by parsing src.
			fileSet := token.NewFileSet() // positions are relative to fset
			file, err := parser.ParseFile(fileSet, goFile, nil, 0)
			if err != nil {
				fmt.Printf("could not parse file: %v\n", err)
				os.Exit(2)
			}

			g.files = append(g.files, file)
		}
	}

	return nil
}

func (g *Generator) isTarget(name string) bool {
	for _, typeName := range g.typeNames {
		if name == typeName {
			return true
		}
	}
	return false
}

// Generator holds the state of the analysis. Primarily used to buffer
// the output for format.Source.
type Generator struct {
	packageDir string
	files      []*ast.File
	Targets    []*TargetType
	typeSpecs  map[string]*TypeSpecNode

	typeNames      []string
	methods        []string
	excludedFields []string
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
	err = g.write(&buf, templateFile)
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

func (g *Generator) write(w io.Writer, file embed.FS) error {
	if len(g.Targets) < 1 {
		return errors.New("generate.render.write: no targets found")
	}
	tmpl, err := template.ParseFS(file, "*")
	if err != nil {
		return errors.New(fmt.Sprintf("generate.render.write: %v", err))
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

type TargetField struct {
	Name     string
	Field    *ast.Field
	TypeName string
	Kind     string

	KeyType   *TargetField // the type of a map key
	ValueType *TargetField // the type of a map or array value

	isCopier bool // does this type implement Copy
	g        *Generator
}

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

func (f *TargetField) resolveType(node ast.Node) bool {
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

					var elemTypeName string
					var ident string
					var kind string
					expr, ok := t.Elt.(*ast.StarExpr)
					if ok {
						ident = expr.X.(*ast.Ident).Name
						elemTypeName = "*" + ident
						kind = "pointer"
					} else {
						ident = t.Elt.(*ast.Ident).Name
						elemTypeName = ident
						kind = ident
					}

					ts := f.g.typeSpecs[ident]

					f.ValueType = &TargetField{
						Kind:     kind,
						TypeName: elemTypeName,
						isCopier: ts != nil && ts.isCopier(),
						g:        f.g,
					}

				case *ast.MapType:
					f.Kind = "map"
					var valueTypeName string
					var ident string
					var kind string

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

					ts := f.g.typeSpecs[ident]
					f.ValueType = &TargetField{
						Kind:     kind,
						TypeName: valueTypeName,
						isCopier: ts != nil && ts.isCopier(),
						g:        f.g,
					}
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

type TargetType struct {
	Name           string // Name of the type we're generating methods for
	methods        []string
	excludedFields []string
	Fields         []*TargetField
	g              *Generator
}

func (t *TargetType) Abbr() string {
	return strings.ToLower(string(t.Name[0]))
}

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

func (t *TargetType) IsCopy() bool {
	return t.hasMethod("copy")
}

func (t *TargetType) IsEquals() bool {
	return t.hasMethod("equals")
}

func (t *TargetType) IsDiff() bool {
	return t.hasMethod("diff")
}

func (t *TargetType) IsMerge() bool {
	return t.hasMethod("merge")
}

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
				ast.Inspect(field, targetField.resolveType)
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
