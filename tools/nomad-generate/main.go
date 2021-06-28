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
	// TODO: See which of these we really need
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

// evaluateTarget traverses a generic declaration node to determine if it is a
// struct, and if so whether it is a struct we are targeting.
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
