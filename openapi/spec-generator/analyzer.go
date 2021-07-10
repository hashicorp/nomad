package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"go/types"
	"golang.org/x/tools/go/cfg"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/types/typeutil"
	"reflect"
	"strings"
)

type HTTPProfile struct {
	IsResponseWriter bool // net/http.ResponseWriter
	IsRequest        bool // *net/http.Request
	IsHandler        bool // net/http.Handler
}

func NewAnalyzer(configs []*PackageConfig, logger loggerFunc, debugOptions DebugOptions) (*Analyzer, error) {
	var err error
	analyzer := &Analyzer{
		PackageConfigs: configs,
		Logger:         logger,
		debugOptions:   debugOptions,
		Packages:       make(map[string]*packages.Package),
	}

	//if err := analyzer.buildProgram(); err != nil {
	//	return nil, err
	//}

	for _, pkgConfig := range analyzer.PackageConfigs {
		var pkgs []*packages.Package
		if pkgs, err = packages.Load(&pkgConfig.Config, pkgConfig.Pattern); err != nil {
			return nil, fmt.Errorf("NewAnalyzer.packages.Load: %v", err)
		}

		for _, pkg := range pkgs {
			analyzer.Packages[pkg.Name] = pkg
			analyzer.typesInfos = append(analyzer.typesInfos, pkg.TypesInfo)
		}
	}

	if err = analyzer.loadTypeObjects(); err != nil {
		return nil, err
	}
	return analyzer, nil
}

// Analyzer holds the SSA && AST state and provides a number of static analysis helper functions
type Analyzer struct {
	PackageConfigs []*PackageConfig
	Logger         loggerFunc
	Packages       map[string]*packages.Package
	debugOptions   DebugOptions
	prog           *ssa.Program
	typesInfos     []*types.Info
	typeObjects    map[string]types.Object
}

func (a *Analyzer) loadTypeObjects() error {
	a.typeObjects = make(map[string]types.Object)

	for _, typeInfo := range a.typesInfos {
		a.loadIdentMapTypeObjects(typeInfo.Defs)
		a.loadIdentMapTypeObjects(typeInfo.Uses)
	}
	return nil
}

func (a *Analyzer) loadIdentMapTypeObjects(identMap map[*ast.Ident]types.Object) {
	for _, v := range identMap {
		switch typeNameDef := v.(type) {
		case *types.TypeName:
			typeKey := a.FormatTypeObjectKey(v, typeNameDef)
			if _, ok := a.typeObjects[typeKey]; !ok {
				a.typeObjects[typeKey] = v
			}
		}
	}
}

func (a *Analyzer) FormatTypeObjectKey(typesObj types.Object, typeNameDef types.Object) string {
	typeKey := typesObj.Id()
	if typeNameDef.Pkg() != nil {
		typeKey = a.FormatTypeName(typeNameDef.Pkg().Name(), typeKey)
	}
	return typeKey
}

func (a *Analyzer) Types(expr ast.Expr) *types.TypeAndValue {
	for _, typesInfo := range a.typesInfos {
		if tv, ok := typesInfo.Types[expr]; ok {
			return &tv
		}
	}

	return nil
}

func (a *Analyzer) Uses(ident *ast.Ident) types.Object {
	for _, typesInfo := range a.typesInfos {
		if tv, ok := typesInfo.Uses[ident]; ok {
			return tv
		}
	}

	return nil
}

func (a *Analyzer) Selections(expr *ast.SelectorExpr) *types.Selection {
	for _, typesInfo := range a.typesInfos {
		if sel, ok := typesInfo.Selections[expr]; ok {
			return sel
		}
	}

	return nil
}

func (a *Analyzer) Defs(ident *ast.Ident) types.Object {
	if ident == nil {
		return nil
	}

	for _, typesInfo := range a.typesInfos {
		if def, ok := typesInfo.Defs[ident]; ok {
			return def
		}
	}

	if a.debugOptions.printDefs {
		a.printDefs()
	}

	return nil
}

func (a *Analyzer) printDefs() {
	for _, typesInfo := range a.typesInfos {
		for k, v := range typesInfo.Defs {
			a.Logger(k.String(), ": ", v)
			if k.Obj != nil {
				a.Logger(k.String(), ": Obj.Name = ", k.Obj.Name)
			}
			if v != nil {
				a.Logger(k.String(), ": Value Name ", v.Name())
				a.Logger(k.String(), ": Value Id ", v.Id())
				a.Logger(k.String(), ": Value Type ", v.Type().String())
				if v.Type().Underlying() != nil {
					a.Logger(k.String(), ": Value Underlying Type ", v.Type().Underlying().String())
				}
			}
		}
	}
}

// GetSource returns the source code for an ast.Node
func (a *Analyzer) GetSource(elem interface{}, fileSet *token.FileSet) (string, error) {
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fileSet, elem); err != nil {
		return "", err
	} else {
		return buf.String(), nil
	}
}

func (a *Analyzer) IsHttpHandler(typeDefFunc *types.Func) bool {
	if funcSignature, ok := typeDefFunc.Type().(*types.Signature); ok {
		// typeDefFunc.
		profile := HTTPProfile{}

		a.setHTTPProfile(funcSignature.Params(), &profile)
		a.setHTTPProfile(funcSignature.Results(), &profile)

		return profile.IsHandler || (profile.IsResponseWriter && profile.IsRequest)
	}

	return false
}

func (a *Analyzer) setHTTPProfile(tup *types.Tuple, result *HTTPProfile) {
	if tup == nil {
		return
	}

	for i := 0; i < tup.Len(); i++ {
		tupleMember := tup.At(i)
		objectType := tupleMember.Type().String()
		switch objectType {
		case "net/http.ResponseWriter":
			result.IsResponseWriter = true
		case "*net/http.Request":
			result.IsRequest = true
		case "net/http.Handler":
			result.IsHandler = true
		default:
			// capture cases such as function that return or accept functions
			// ex. (func(net/http.Handler) net/http.Handler, error)
			if strings.Contains(objectType, "net/http.ResponseWriter") {
				result.IsResponseWriter = true
			}
			if strings.Contains(objectType, "*net/http.Request") {
				result.IsRequest = true
			}
			if strings.Contains(objectType, "net/http.Handler") {
				result.IsHandler = true
			}
		}
	}

	return
}

func (a *Analyzer) GetHttpHandlers(pkg *packages.Package) map[string]*types.Func {
	httpHandlers := make(map[string]*types.Func)
	for _, typeDef := range pkg.TypesInfo.Defs {
		if typeDef != nil {
			if typeDefFunc, ok := typeDef.(*types.Func); ok {
				if a.IsHttpHandler(typeDefFunc) {
					httpHandlers[fmt.Sprintf("%s.%s", pkg.Name, typeDefFunc.Name())] = typeDefFunc
				}
			}
		}
	}

	return httpHandlers
}

func (a *Analyzer) GetStructs(pkg *packages.Package) (map[string]*ast.TypeSpec, error) {
	var structMap = make(map[string]*ast.TypeSpec)
	fmtString := pkg.Name + ".%s"

	visitFunc := func(node ast.Node) bool {
		switch typeSpec := node.(type) {
		case *ast.TypeSpec:
			switch typeSpec.Type.(type) {
			case *ast.StructType:
				structMap[fmt.Sprintf(fmtString, typeSpec.Name.Name)] = typeSpec
				structSpec, _ := typeSpec.Type.(*ast.StructType)
				// Check each of the types fields to see if it is a pointer to a type.
				for _, field := range structSpec.Fields.List {
					switch fieldType := field.Type.(type) {
					case *ast.StructType:
						fmt.Println(fmt.Sprintf("%v", field))
						//structMap[fmt.Sprintf(fmtString)] = typeSpec
					case *ast.MapType:
					case *ast.ArrayType:
					case *ast.StarExpr:
						// If it is a pointer, figure out the underlying type.
						ident, ok := fieldType.X.(*ast.Ident)
						if ok {
							// check to see if it's been registered already
							if _, ok = structMap[fmt.Sprintf(fmtString, ident.Name)]; !ok && ident.Obj != nil {
								var objTypeSpec *ast.TypeSpec
								if objTypeSpec, ok = ident.Obj.Decl.(*ast.TypeSpec); ok {
									structMap[fmt.Sprintf(fmtString, ident.Name)] = objTypeSpec
								}
							}
						}
					}
				}

			}
		}
		return true
	}

	for _, goFile := range pkg.GoFiles {
		fileSet := token.NewFileSet() // positions are relative to fset
		file, err := parser.ParseFile(fileSet, goFile, nil, 0)
		if err != nil {
			return nil, fmt.Errorf("Analyzer.GetStructs.parser.ParseFile: %v\n", err)
		}

		ast.Inspect(file, visitFunc)
	}

	return structMap, nil
}

func (a *Analyzer) GetControlFlowGraph(fn *types.Func, decl *ast.FuncDecl) *cfg.CFG {
	for _, pkg := range a.Packages {
		c := cfg.New(decl.Body, a.callMayReturn(pkg, fn, decl))
		if c != nil {
			return c
		}
	}
	return nil
}

var panicBuiltin = types.Universe.Lookup("panic").(*types.Builtin)

func (a *Analyzer) callMayReturn(pkg *packages.Package, fn *types.Func, decl *ast.FuncDecl) func(call *ast.CallExpr) bool {
	return func(call *ast.CallExpr) bool {
		if id, ok := call.Fun.(*ast.Ident); ok && pkg.TypesInfo.Uses[id] == panicBuiltin {
			return false // panic never returns
		}

		// Is this a static call?
		fn := typeutil.StaticCallee(pkg.TypesInfo, call)
		if fn == nil {
			return true // callee not statically known; be conservative
		}

		return !isIntrinsicNoReturn(fn)
	}
}

func isIntrinsicNoReturn(fn *types.Func) bool {
	// Add functions here as the need arises, but don't allocate memory.
	path, name := fn.Pkg().Path(), fn.Name()
	return path == "syscall" && (name == "Exit" || name == "ExitProcess" || name == "ExitThread") ||
		path == "runtime" && name == "Goexit"
}

func (a *Analyzer) GetFieldType(fieldName string, obj types.Object) types.Object {
	if s, ok := obj.Type().(*types.Named); ok {
		if orig, ok := s.Underlying().(*types.Struct); ok {
			for i := 0; i < orig.NumFields(); i++ {
				field := orig.Field(i)
				if field.Name() == fieldName {
					return field
				}
			}
		}
	}

	return nil
}

func (a *Analyzer) GetSliceElemType(obj types.Object) types.Object {
	sliceType, ok := obj.Type().(*types.Slice)
	if !ok {
		panic(fmt.Sprintf("Analyzer.GetSliceElemType invalid type %v", obj.Type()))
	}

	switch elemType := sliceType.Elem().(type) {
	case *types.Pointer:
		if obj, ok := elemType.Elem().(types.Object); !ok {
			panic("Analyzer.GetSliceElemType invalid cast")
		} else {
			return obj
		}
	}

	return nil
}

func (a *Analyzer) IsSlice(obj types.Object) bool {
	_, ok := obj.Type().(*types.Slice)
	return ok
}

func (a *Analyzer) NewFromTypeObj(outObject types.Object) (interface{}, error) {
	reflectType, err := a.ToReflectType(outObject.Type())
	if err != nil {
		return nil, err
	}

	iface := reflect.New(reflectType).Elem().Addr().Interface()
	return iface, nil
}

func (a *Analyzer) ToStruct(obj *types.Struct) (interface{}, error) {

	fields := make([]reflect.StructField, 0)

	for i := 0; i < obj.NumFields(); i++ {
		field, err := a.ToStructField(obj.Field(i))
		if err != nil {
			return nil, err
		}
		fields = append(fields, field)
	}

	typ := reflect.StructOf(fields)
	v := reflect.New(typ).Elem()

	return v.Addr().Interface(), nil
}

func (a *Analyzer) ToStructField(varField *types.Var) (reflect.StructField, error) {
	fieldType, err := a.ToReflectType(varField.Type())
	if err != nil {
		return reflect.StructField{}, err
	}

	field := reflect.StructField{
		Name: varField.Name(),
		Tag:  reflect.StructTag(fmt.Sprintf(`json:"%s"`, varField.Name())),
		Type: fieldType,
	}

	return field, nil
}

func (a *Analyzer) ToReflectType(t types.Type) (reflect.Type, error) {
	if basic, ok := t.(*types.Basic); ok {
		switch basic.Kind() {
		case types.Bool, types.UntypedBool:
			return reflect.TypeOf(true), nil
		case types.Int, types.Int8, types.Int16, types.Int32, types.Int64, types.Uint, types.Uint8, types.Uint16, types.Uint32, types.Uint64, types.UntypedInt, types.UntypedRune:
			return reflect.TypeOf(0), nil
		case types.Float32, types.Float64, types.UntypedFloat:
			return reflect.TypeOf(0.0), nil
		case types.Complex64, types.UntypedComplex:
			return reflect.TypeOf(complex64(1)), nil
		case types.Complex128:
			return reflect.TypeOf(complex128(4)), nil
		case types.String, types.UntypedString:
			return reflect.TypeOf("str"), nil
		default:
			return nil, fmt.Errorf("Anazlyzer.ToReflectType unhandled basic kind %v", basic)
		}
	}

	switch typesType := t.(type) {
	case *types.Named:
		typ, err := a.ToReflectType(typesType.Underlying())
		if err != nil {
			return nil, err
		}
		return typ, nil
	case *types.Struct:
		instance, err := a.ToStruct(typesType)
		if err != nil {
			return nil, err
		}
		return reflect.TypeOf(instance), nil
	case *types.Slice:
		var err error
		var elem types.Type
		var elemType reflect.Type
		elem = typesType.Elem()
		switch switchType := elem.(type) {
		case *types.Pointer:
			elemType, err = a.ToReflectType(switchType.Elem())
			if err != nil {
				return nil, err
			}
		}
		elemType, err = a.ToReflectType(elem)
		if err != nil {
			return nil, err
		}
		return reflect.SliceOf(elemType), nil
	case *types.Array:
		elem, err := a.ToReflectType(typesType.Elem())
		if err != nil {
			return nil, err
		}
		return reflect.ArrayOf(0, elem), nil
	case *types.Map:
		keyType, err := a.ToReflectType(typesType.Key())
		if err != nil {
			return nil, err
		}
		elemType, err := a.ToReflectType(typesType.Elem())
		if err != nil {
			return nil, err
		}
		return reflect.MapOf(keyType, elemType), nil
	case *types.Pointer:
		return a.ToReflectType(typesType.Elem())
	}

	return nil, fmt.Errorf(fmt.Sprintf("Analyzer.ToReflectType unhandled type %v", t))
}
