package main

import (
	"errors"
	"fmt"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3gen"
	"github.com/ghodss/yaml"
	"go/ast"
	"go/token"
	"go/types"
	"golang.org/x/tools/go/cfg"
	"golang.org/x/tools/go/packages"
	"math"
	"net/http"
	"sort"
	"strings"
	"sync"
)

func newVarAdapter(spec *ast.ValueSpec, analyzer *Analyzer, source string) *varAdapter {
	return &varAdapter{
		spec,
		analyzer,
		source,
	}
}

type varAdapter struct {
	spec     *ast.ValueSpec
	analyzer *Analyzer
	source   string // @@@@ Debug useful, shouldn't ultimately be needed.
}

func (v *varAdapter) getPackageName() string {
	if strings.Contains(v.Name(), ".") {
		return ""
	}

	charAfterIndex := strings.Index(v.source, v.Name()) + len(v.Name()) + 1

	splitSource := strings.Split(v.source[charAfterIndex:], " ")
	return splitSource[0]
}

func (v *varAdapter) Name() string {
	return v.spec.Names[0].Name
}

func (v *varAdapter) TypeName() string {
	return v.GetTypeNameFromSpec()
	// return v.analyzer.FormatTypeName(v.getPackageName(), v.GetTypeNameFromSpec())
}

func (v *varAdapter) GetTypeNameFromSpec() string {
	switch specType := v.spec.Type.(type) {
	case *ast.Ident:
		return specType.Name
	case *ast.StarExpr:
		switch xType := specType.X.(type) {
		case *ast.Ident:
			return xType.Name
		case *ast.SelectorExpr:
			return v.analyzer.FormatTypeName(xType.X.(*ast.Ident).Name, xType.Sel.Name)
		default:
			v.analyzer.Logger("varAdapter.GetTypeNameFromSpec unhandled type", specType)
		}
	case *ast.SelectorExpr:
		return v.analyzer.FormatTypeName(specType.X.(*ast.Ident).Name, specType.Sel.Name)
	//case *ast.ArrayType:
	default:
		v.analyzer.Logger("varAdapter.GetTypeNameFromSpec: unhandled type", specType)
	}

	return "unknown"
}

func (v *varAdapter) Type() types.Object {
	return v.analyzer.GetTypeByName(v.TypeName(), v.spec.Pos())
}

type pathItemAdapter struct {
	method  string // GET, PUT etc.
	Handler *handlerFuncAdapter
}

// GetMethod returns a string that maps to the net/http method this pathItemAdapter
// represents e.g. GET, POST, PUT
func (pia *pathItemAdapter) GetMethod() string {
	method := "unknown"

	return method
}

// GetInputParameterRefs creates an ParameterRef slice by inspecting the source code
func (pia *pathItemAdapter) GetInputParameterRefs() []*openapi3.ParameterRef {
	var refs []*openapi3.ParameterRef

	//for _, param := range t.Object.Params.List {
	//	params = fmt.Sprintf("%s|%s ", param.Names[0].Name, param.Object)
	//}

	return refs
}

// GetRequestBodyRef creates a RequestBodyRef by inspecting the source code
func (pia *pathItemAdapter) GetRequestBodyRef() *openapi3.RequestBodyRef {
	ref := &openapi3.RequestBodyRef{}

	return ref
}

// GetResponseSchemaRef creates a SchemaRef by inspecting the source code. This
// is intended as a debug function. Use GetResponseRefs to generate a spec.
func (pia *pathItemAdapter) GetResponseSchemaRef() *openapi3.SchemaRef {
	ref := &openapi3.SchemaRef{}

	return ref
}

// GetResponseRefs creates a slice of ResponseRefs by inspecting the source code
func (pia *pathItemAdapter) GetResponseRefs() []*openapi3.ResponseRef {
	var refs []*openapi3.ResponseRef

	return refs
}

type handlerFuncAdapter struct {
	Package  *packages.Package
	Func     *types.Func
	FuncDecl *ast.FuncDecl

	handlerName string
	logger      loggerFunc
	// TODO: Figure out the right model and naming. Analyzer is a loaded term.
	// Having 2 stateful things feels weird, yet the Analyzer shouldn't be aware
	// of OpenAPI.
	// This is stateful and needs to be shared everywhere
	analyzer *Analyzer
	// This is stateful and needs to be shared everywhere
	schemaRefAdapter *schemaRefAdapter
	fileSet          *token.FileSet

	Variables map[string]*varAdapter
	// The CFG does contain Return statements; even implicit returns are materialized
	// (at the position of the function's closing brace).
	// CFG does not record conditions associated with conditional branch edges,
	//nor the short-circuit semantics of the && and || operators, nor abnormal
	//control flow caused by panic. If you need this information, use golang.org/x/tools/go/ssa instead.
	Cfg *cfg.CFG
}

func (h *handlerFuncAdapter) GetPath() string {
	// TODO: Resolve the path
	return "/" + h.Name()
}

// TODO: Find a way to make this injectable
var supportedMethods = []string{http.MethodGet, http.MethodDelete, http.MethodPut, http.MethodPost}

func (h *handlerFuncAdapter) newPathItemAdapter(method string) (*pathItemAdapter, error) {
	isSupportedMethod := false
	for _, supportedMethod := range supportedMethods {
		if supportedMethod == method {
			isSupportedMethod = true
		}
	}
	if !isSupportedMethod {
		return nil, fmt.Errorf("HandlerFuncAdapter.newPathItemAdapter: method %s not supported", method)
	}

	return &pathItemAdapter{method: method}, nil
}

func (h *handlerFuncAdapter) Name() string {
	return h.Func.Name()
}

func (h *handlerFuncAdapter) GetSource() (string, error) {
	if h.FuncDecl == nil {
		h.analyzer.Logger("FuncDecl nil for", h.Name())
		return "", nil
	}
	src, err := h.analyzer.GetSource(h.FuncDecl.Body, h.fileSet)
	if err != nil {
		return "", err
	}
	return src, nil
}

func (h *handlerFuncAdapter) debugReturnSource(idx int) string {
	result, _ := h.GetResultByIndex(idx)
	src, _ := h.analyzer.GetSource(result, h.fileSet)
	src = strings.Replace(src, "\n", "", -1)
	src = strings.Replace(src, "\t", "", -1)
	return src
}

// IsIntermediateFunc is used to determine if an HTTP Handler is actually
// an intermediate function that in turn calls other functions to handle
// different HTTP methods, path parameters, etc.
func (h *handlerFuncAdapter) IsIntermediateFunc() bool {
	// TODO: Find a way to detect that this is a FooSpecificRequestFunc
	return false
}

// visitHandlerFunc makes several passes over the Handler FuncDecl. Order matters
// and trying to this all in one pass is likely not an option.
func (h *handlerFuncAdapter) visitHandlerFunc() error {
	if err := h.FindVariables(); err != nil {
		return err
	}
	if _, err := h.GetReturnSchema(); err != nil {
		return err
	}

	return nil
}

func (h *handlerFuncAdapter) FindVariables() error {
	if h.Variables == nil {
		h.Variables = make(map[string]*varAdapter)
	}

	source, err := h.GetSource()
	if err != nil {
		return fmt.Errorf("%s.HandlerFuncAdapter.FindVariables could not get source: %#v", h.handlerName, err)
	}

	var variableVisitor = func(node ast.Node) bool {
		switch genDecl := node.(type) {
		case *ast.GenDecl:
			if genDecl.Tok != token.VAR {
				return true
			}

			for _, spec := range genDecl.Specs {
				switch specType := spec.(type) {
				case *ast.ValueSpec:
					varAdapter := newVarAdapter(specType, h.analyzer, source)
					// @@@@ DEBUG
					if varAdapter.Type() == nil {
						h.logger(h.handlerName, "varAdapter.Type() nil for ValueSpecType:", specType)
					}

					if _, ok := h.Variables[varAdapter.Name()]; !ok {
						h.Variables[varAdapter.Name()] = varAdapter
					}
				default:
					h.analyzer.Logger("handlerFuncAdapter.variableVisitor unhandled type", specType)
				}
			}
		}
		return true
	}

	ast.Inspect(h.FuncDecl, variableVisitor)

	if h.analyzer.debugOptions.printVariables {
		for k, v := range h.Variables {
			h.analyzer.Logger(h.handlerName, "variable", k, "has TypeName:", v.TypeName(), "with type signature", v.Type())
		}
	}

	return nil
}

func (h *handlerFuncAdapter) GetReturnSchema() (*openapi3.SchemaRef, error) {
	result, err := h.GetResultByIndex(0)
	if err != nil {
		return nil, err
	}

	var outObject types.Object

	outVisitor := func(node ast.Node) bool {
		// if node is nil or outObject has been resolved exit
		if node == nil || outObject != nil {
			return true
		}
		switch t := node.(type) {
		case *ast.Ident:
			if t.Name == "nil" {
				outObject = nil
				return true
			}

			var v *varAdapter
			var ok bool
			// @@@@ Debug should never happen if all loading is done correctly.
			if v, ok = h.Variables[t.Name]; ok {
				outObject = v.Type()
			}
		case *ast.SelectorExpr:
			// in this case we are returning a field of a variable
			// X should be an Ident and X.Name will have the variable.
			// Sel.Name should have the field name.
			switch xt := t.X.(type) {
			case *ast.Ident:
				var v *varAdapter
				var ok bool
				if v, ok = h.Variables[xt.Name]; !ok {
					h.analyzer.Logger(fmt.Sprintf("%s.HandlerFuncAdapter.GetReturnSchema.outVisitor failed to find variable %s", h.handlerName, xt.Name))
					// @@@@ Debug
					return true
				}
				outObject = h.analyzer.GetFieldType(t.Sel.Name, v.Type())
			default:
				h.analyzer.Logger(fmt.Sprintf("%s.HandlerFuncAdapter.GetReturnSchema: unhandled type %#v", h.handlerName, xt))
			}
		//case *ast.UnaryExpr:
		//case *ast.CompositeLit:
		//case *ast.KeyValueExpr:
		//case *ast.CallExpr:
		//case *ast.BasicLit:
		//case *ast.FuncLit:
		//case *ast.FuncType:
		//case *ast.FieldList:
		//case *ast.Field:
		//case *ast.BlockStmt:
		//case *ast.AssignStmt:
		//case *ast.StarExpr:
		//case *ast.ExprStmt:
		//case *ast.IfStmt:
		//case *ast.BinaryExpr:
		default:
			h.analyzer.Logger(h.handlerName, fmt.Sprintf("HandlerFuncAdapter.GetReturnSchema: unhandled node type %#v", t))
		}
		return true
	}

	ast.Inspect(result, outVisitor)

	if outObject == nil {
		if ident, ok := result.(*ast.Ident); ok {
			if ident.Name != "nil" && ident.Name != "true" && ident.Name != "false" {
				h.analyzer.Logger(h.handlerName, "handlerFuncAdapter.GetReturnSchema evaluated nil for result", result, ident.Obj.Decl)
				return nil, nil
			}
		}
	}

	//schemaRef, err := h.schemaRefAdapter.GetOrCreateSchemaRef(nil, &outObject, "schemas")
	if outObject == nil {
		return nil, nil
	}

	schemaRef, err := h.schemaRefAdapter.generateWithoutSaving(nil, outObject.Type())
	if err != nil {
		return nil, err
	}

	if h.analyzer.debugOptions.printSchemaRefs {
		data, _ := yaml.Marshal(schemaRef)
		h.analyzer.Logger(string(data))
	}
	return schemaRef, nil
}

func (h *handlerFuncAdapter) getReturnStmts() []*ast.ReturnStmt {
	var returnStmts []*ast.ReturnStmt

	returnVisitor := func(node ast.Node) bool {
		switch t := node.(type) {
		case *ast.ReturnStmt:
			returnStmts = append(returnStmts, t)
		}
		return true
	}

	ast.Inspect(h.FuncDecl, returnVisitor)

	return returnStmts
}

func (h *handlerFuncAdapter) GetResultByIndex(idx int) (ast.Expr, error) {
	returnStmts := h.getReturnStmts()
	if len(returnStmts) < 1 {
		return nil, fmt.Errorf("HandlerFuncAdapter.GetResultByIndex: no return statement found")
	}

	finalReturn := returnStmts[len(returnStmts)-1]
	if finalReturn == nil {
		return nil, fmt.Errorf("HandlerFuncAdapter.GetResultByIndex: finalReturn does not exist")
	}

	if len(finalReturn.Results) < idx+1 {
		return nil, fmt.Errorf("HandlerFuncAdapter.GetResultByIndex: invalid index")
	}
	return finalReturn.Results[idx].(ast.Expr), nil
}

// schemaRefAdapter converts interfaces to SchemaRefs, ensuring the same type is
// not registered twice. This is adapted from kin-openapi/openapi3gen. Because our
// struct types are hydrated generically, we won't have type names available in
// in the reflect info. So we have to maintain our own type registry based on the
// typeName we have resolved already.
type schemaRefAdapter struct {
	SchemaRefs       map[string]*openapi3.SchemaRef
	typeObjects      map[string]*types.Object
	typeObjectsMutex sync.RWMutex
	types            map[string]*types.Type
	typesMutex       sync.RWMutex
	analyzer         *Analyzer
}

//func (s *SchemaRefAdapter) GenerateSchemaRefs(iface interface{}, typeName string) (*openapi3.SchemaRef, error) {
//	ref, err := s.GenerateSchemaRef(reflect.TypeOf(iface), typeName)
//	if err != nil {
//		return nil, err
//	}
//
//	return ref, err
//}

func newSchemaRefAdapter(analyzer *Analyzer) *schemaRefAdapter {
	return &schemaRefAdapter{
		SchemaRefs:       make(map[string]*openapi3.SchemaRef),
		typeObjects:      make(map[string]*types.Object),
		typeObjectsMutex: sync.RWMutex{},
		types:            make(map[string]*types.Type),
		typesMutex:       sync.RWMutex{},
		analyzer:         analyzer,
	}
}

func (s *schemaRefAdapter) GetOrCreateSchemaRef(parents []*types.Object, objPtr *types.Object, componentType string) (*openapi3.SchemaRef, error) {
	if objPtr == nil {
		return nil, fmt.Errorf("SchemaRefAdapter.GetOrCreateSchemaRef: objPtr cannot be nil")
	}

	obj := *objPtr
	if obj == nil {
		return nil, fmt.Errorf("SchemaRefAdapter.GetOrCreateSchemaRef: obj cannot be nil")
	}

	if t, ok := obj.Type().(*types.Pointer); ok {
		obj = s.analyzer.GetPointerElem(t)
	}

	typeName := "unknown"
	schema := &openapi3.Schema{}

	if basic, ok := obj.Type().(*types.Basic); ok {
		s.adaptBasic(basic, schema)
		// default to schema.Object for basic kinds
		// TODO: is this ok, or does it need to be based on format
		typeName = schema.Type
	} else {
		switch objType := obj.Type().(type) {
		case *basicKindType:
			s.adaptBasic(objType.basic, schema)
			typeName = schema.Type
		case *types.Slice:
			schema.Type = "array"
			switch elemType := objType.Elem().(type) {
			case *types.Basic:
				basicSchema := &openapi3.Schema{}
				s.adaptBasic(elemType, basicSchema)
				items := &openapi3.SchemaRef{
					Ref:   "",
					Value: basicSchema,
				}
				schema.Items = items
			default:
				itemsElemObj := s.analyzer.GetSliceElemObj(obj)
				typeName = itemsElemObj.Name() + "Array"
				items, err := s.GetOrCreateSchemaRef(parents, &itemsElemObj, componentType)
				if err != nil {
					s.analyzer.Logger("schemaRefAdapter.GetOrCreateSchema failed for slice item", itemsElemObj.Name())
					return nil, err
				}
				if items != nil {
					if !s.addSchemaRef(typeName, items) {
						return nil, fmt.Errorf("schemaRefAdapter.GetOrCreateScheam unable to add schemaRef for slice elem" + typeName)
					}
					schema.Items = items
				}
			}
		case *types.Map:
			schema.Type = "object"
			var elemObj types.Object
			if elemObj, ok = objType.Elem().(types.Object); !ok {
				err := errors.New(fmt.Sprintf("SchemaRefAdapter.GetOrCreateSchemaRef: invalid map type %#v", objType))
				s.analyzer.Logger(err)
				return nil, err
			}
			additionalProperties, err := s.GetOrCreateSchemaRef(parents, &elemObj, componentType)
			if err != nil {
				s.analyzer.Logger("schemaRefAdapter.GetOrCreateSchema failed for map item", elemObj.Name())
				return nil, err
			}
			if additionalProperties != nil {
				if !s.addSchemaRef(elemObj.Name(), additionalProperties) {
					return nil, fmt.Errorf("schemaRefAdapter.GetOrCreateSchema unable to add schemaRef for additionalProperties " + elemObj.Name())
				}
				schema.AdditionalProperties = additionalProperties
			}

			typeName = elemObj.Name() + "Map"
		case *types.Named:
			typeName = objType.Obj().Name()
			switch underlyingType := objType.Underlying().(type) {
			case *types.Basic:
				// Do nothing, don't look for
				typeName = ""
			case *types.Struct:
				err := s.adaptStruct(parents, objPtr, schema, underlyingType, componentType)
				if err != nil {
					return nil, err
				}
			//case *types.Slice:
			//	err := s.adaptStruct(parents, objPtr, schema, underlyingType.Elem(), componentType)
			//	if err != nil {
			//		return nil, err
			//	}
			default:
				err := errors.New(fmt.Sprintf("SchemaRefAdapter.GetOrCreateSchemaRef map.Named.Underlying failed to handle type: %#v", underlyingType))
				return nil, err
			}
		case *types.Struct:
			typeName = obj.(types.Object).Name()
			err := s.adaptStruct(parents, objPtr, schema, objType, componentType)
			if err != nil {
				return nil, err
			}
		default:
			s.analyzer.Logger("schemaRefAdapter.GetOrCreateSchema unhandled type", objType)
		}
	}

	if existing, ok := s.SchemaRefs[typeName]; ok {
		return existing, nil
	}

	// Don't set the Ref value if you want the serializer to dump the full schema.
	ref := openapi3.NewSchemaRef("", schema)
	if !s.addSchemaRef(typeName, ref) {
		return nil, fmt.Errorf("schemaRefAdapter unable to add schemaRef for " + typeName)
	}
	return ref, nil
}

func (s *schemaRefAdapter) addSchemaRef(key string, items *openapi3.SchemaRef) bool {
	var ok bool
	if len(key) < 1 {
		return false
	}
	if _, ok = s.SchemaRefs[key]; !ok {
		s.SchemaRefs[key] = items
	}
	return true
}

func (s *schemaRefAdapter) adaptBasic(basic *types.Basic, schema *openapi3.Schema) {
	switch basic.Kind() {
	case types.Bool:
		schema.Type = "boolean"
	case types.Int:
		schema.Type = "integer"
	case types.Int8:
		schema.Type = "integer"
		schema.Min = &minInt8
		schema.Max = &maxInt8
	case types.Int16:
		schema.Type = "integer"
		schema.Min = &minInt16
		schema.Max = &maxInt16
	case types.Int32:
		schema.Type = "integer"
		schema.Format = "int32"
	case types.Int64:
		schema.Type = "integer"
		schema.Format = "int64"
	case types.Uint:
		schema.Type = "integer"
		schema.Min = &zeroInt
	case types.Uint8:
		if strings.Contains(basic.String(), "byte") {
			schema.Type = "string"
			schema.Format = "byte"
		} else {
			schema.Type = "integer"
			schema.Min = &zeroInt
			schema.Max = &maxUint8
		}
	case types.Uint16:
		schema.Type = "integer"
		schema.Min = &zeroInt
		schema.Max = &maxUint16
	case types.Uint32:
		schema.Type = "integer"
		schema.Min = &zeroInt
		schema.Max = &maxUint32
	case types.Uint64:
		schema.Type = "integer"
		schema.Min = &zeroInt
		schema.Max = &maxUint64
	case types.Float32:
		schema.Type = "number"
		schema.Format = "float"
	case types.Float64:
		schema.Type = "number"
		schema.Format = "double"
	case types.String:
		schema.Type = "string"
	default:
		s.analyzer.Logger("schemaRefAdapter.adaptBasic unhandled kind", basic)
	}
}

func (s *schemaRefAdapter) adaptStruct(parents []*types.Object, objPtr *types.Object, schema *openapi3.Schema, structType *types.Struct, componentType string) error {
	obj := *objPtr
	if obj.Name() == "Time" {
		schema.Type = "string"
		schema.Format = "date-time"
		return nil
	}

	if schema.Extensions == nil {
		schema.Extensions = make(map[string]interface{})
	}

	schema.Extensions["x-go-package"] = obj.Pkg().Path()

	// Check for circular reference
	objPtr = s.getTypesObject(objPtr)
	for _, parent := range parents {
		if parent == objPtr {
			return &openapi3gen.CycleError{}
		}
	}
	if cap(parents) == 0 {
		parents = make([]*types.Object, 0, 4)
	}
	parents = append(parents, objPtr)

	// Sort the fields for easier debugging and unit tests
	var fields []*types.Var
	for i := 0; i < structType.NumFields(); i++ {
		fields = append(fields, structType.Field(i))
	}
	sort.Slice(fields, func(i, j int) bool {
		return fields[i].Name() > fields[j].Name()
	})

	for _, field := range fields {
		fieldTypeName := "unknown" // make sure this string never shows up in tests.
		var ref *openapi3.SchemaRef
		var err error

		var propertySchema *openapi3.Schema
		switch fieldType := field.Type().(type) {
		case *types.Basic:
			propertySchema = &openapi3.Schema{}
			s.adaptBasic(fieldType, propertySchema)
		case *types.Pointer:
			if elemType, ok := fieldType.Elem().(*types.Basic); ok {
				propertySchema = &openapi3.Schema{}
				s.adaptBasic(elemType, propertySchema)
				break
			}
			elemType := s.analyzer.GetPointerElem(fieldType)
			fieldTypeName = elemType.Name()
			ref, err = s.GetOrCreateSchemaRef(parents, &elemType, componentType)
			if err != nil {
				s.analyzer.Logger("schemaRefAdapter.adaptStruct case *types.Pointer failed for", obj.Name(), field.Name())
				return err
			}
		case *types.Struct:
			underlying := fieldType.Underlying().(types.Object)
			fieldTypeName = underlying.Name()
			ref, err = s.GetOrCreateSchemaRef(parents, &underlying, componentType)
			if err != nil {
				s.analyzer.Logger("schemaRefAdapter.adaptStruct case *types.Struct failed for", obj.Name(), field.Name())
				return err
			}
		case *types.Named:
			fieldTypeName = fieldType.Obj().Name()
			var indirectObj types.Object
			indirectObj = fieldType.Obj()
			ref, err = s.GetOrCreateSchemaRef(parents, &indirectObj, componentType)
			if err != nil {
				s.analyzer.Logger("schemaRefAdapter.adaptStruct case *types.Named failed for", obj.Name(), field.Name())
				return err
			}
		case *types.Slice:
			propertySchema = &openapi3.Schema{
				Type: "array",
			}
			itemsSchema := &openapi3.Schema{}
			var refPath string
			switch elemType := fieldType.Elem().(type) {
			case *types.Basic:
				s.adaptBasic(elemType, itemsSchema)
				refPath = ""
			case *types.Pointer:
				itemsSchema.Type = "object"
				itemsObj := s.analyzer.GetPointerElem(elemType)
				if itemsObj.Name() == "ServerMember" {
					s.analyzer.Logger("Blick")
				}
				refPath = fmt.Sprintf("#/components/%s/%s", componentType, itemsObj.Name())
				ref, err = s.GetOrCreateSchemaRef(nil, &itemsObj, componentType)
				if err != nil {
					s.analyzer.Logger("schemaRefAdapter.adaptStruct case *types.Slice.Pointer failed for", obj.Name(), field.Name())
					return err
				}
				itemsSchema.Items = ref
			case *types.Struct:
				itemsSchema.Type = "object"
				underlying := elemType.Underlying().(types.Object)
				ref, err = s.GetOrCreateSchemaRef(nil, &underlying, componentType)
				if err != nil {
					s.analyzer.Logger("schemaRefAdapter.adaptStruct case *types.Slice.Struct failed for", obj.Name(), field.Name())
					return err
				}
				itemsSchema.Items = ref
			case *types.Named:
				itemsSchema.Type = "object"
				switch elemType.Underlying().(type) {
				case *types.Interface:
					s.analyzer.Logger("SchemaRefAdapter.adaptStruct unhandled interface", elemType)
				case *types.Struct:
					var indirectObj types.Object
					indirectObj = elemType.Obj()
					ref, err = s.GetOrCreateSchemaRef(nil, &indirectObj, componentType)
					if err != nil {
						s.analyzer.Logger("schemaRefAdapter.adaptStruct case *types.Slice.Named failed for", obj.Name(), field.Name())
						return err
					}
				default:
					s.analyzer.Logger("schemaRefAdapter.adaptStruct: unhandled type slice.Named", elemType.Underlying())
				}
				itemsSchema.Items = ref
			case *types.Map:
				s.analyzer.Logger("schemaRefAdapter.adaptStruct: unhandled type", elemType, "for", obj.Name(), field.Name())
			default:
				s.analyzer.Logger("schemaRefAdapter.adaptStruct *types.Sliced unhandled type", elemType, "for", obj.Name(), field.Name())
			}
			propertySchema.Items = openapi3.NewSchemaRef(refPath, itemsSchema)
		case *types.Map:
			propertySchema = &openapi3.Schema{
				Type: "object",
			}
			itemsSchema := &openapi3.Schema{}
			switch elemType := fieldType.Elem().(type) {
			case *types.Basic:
				basicSchema := &openapi3.Schema{}
				s.adaptBasic(elemType, basicSchema)
				propertySchema.AdditionalProperties = openapi3.NewSchemaRef("", basicSchema)
			case *types.Pointer:
				itemsSchema.Type = "object"
				itemsObj := s.analyzer.GetPointerElem(elemType)
				ref, err = s.GetOrCreateSchemaRef(nil, &itemsObj, componentType)
				if err != nil {
					s.analyzer.Logger("schemaRefAdapter.adaptStruct case *types.Map.Pointer failed for", obj.Name(), field.Name())
					return err
				}
				itemsSchema.Items = ref
			case *types.Struct:
				itemsSchema.Type = "object"
				underlying := elemType.Underlying().(types.Object)
				ref, err = s.GetOrCreateSchemaRef(nil, &underlying, componentType)
				if err != nil {
					s.analyzer.Logger("schemaRefAdapter.adaptStruct case *types.Map.Struct failed for", obj.Name(), field.Name())
					return err
				}
				itemsSchema.Items = ref
			case *types.Named:
				itemsSchema.Type = "object"
				var indirectObj types.Object
				indirectObj = elemType.Obj()
				ref, err = s.GetOrCreateSchemaRef(nil, &indirectObj, componentType)
				if err != nil {
					s.analyzer.Logger("schemaRefAdapter.adaptStruct case *types.Map.Named failed for", obj.Name(), field.Name())
					return err
				}
				itemsSchema.Items = ref
			case *types.Map:
				// TODO: Is there a way to genericize and recursify this
				itemsSchema.Type = "object"
				switch itemsType := elemType.Elem().(type) {
				case *types.Basic:
					basicSchema := &openapi3.Schema{}
					s.adaptBasic(itemsType, basicSchema)
					itemsSchema.Items = openapi3.NewSchemaRef("", basicSchema)
				case *types.Pointer:
					itemsObj := s.analyzer.GetPointerElem(itemsType)
					ref, err = s.GetOrCreateSchemaRef(nil, &itemsObj, componentType)
					if err != nil {
						s.analyzer.Logger("schemaRefAdapter.adaptStruct case *types.Map.Map.Pointer failed for", obj.Name(), field.Name(), itemsObj.Name())
						return err
					}
					itemsSchema.Items = ref
				case *types.Struct:
					underlying := itemsType.Underlying().(types.Object)
					ref, err = s.GetOrCreateSchemaRef(nil, &underlying, componentType)
					if err != nil {
						s.analyzer.Logger("schemaRefAdapter.adaptStruct case *types.Map.Map.Struct failed for", obj.Name(), field.Name(), underlying.Name())
						return err
					}
					itemsSchema.Items = ref
					propertySchema.Items = openapi3.NewSchemaRef("", itemsSchema)
				case *types.Named:
					var indirectObj types.Object
					indirectObj = itemsType.Obj()
					ref, err = s.GetOrCreateSchemaRef(nil, &indirectObj, componentType)
					if err != nil {
						s.analyzer.Logger("schemaRefAdapter.adaptStruct case *types.Map.Map.Named failed for", obj.Name(), field.Name(), indirectObj.Name())
						return err
					}
					itemsSchema.Items = ref
					propertySchema.Items = openapi3.NewSchemaRef("", itemsSchema)
				// case *types.Map:
				default:
					s.analyzer.Logger("schemaRefAdapter.adaptStruct case *types.Map unhandled type", itemsType, "for", obj.Name(), field.Name())
				}
			default:
				s.analyzer.Logger(fmt.Sprintf("SchemaRefAdapter.adaptStruct: struct %s has unhandled field %s of Type %#v", obj.Name(), field.Name(), fieldType))
			}
		}

		if propertySchema != nil {
			if fieldTypeName == "unknown" {
				fieldTypeName = propertySchema.Type
			}
			schema.WithProperty(field.Name(), propertySchema)
		}

		if ref != nil {
			if fieldTypeName == "unknown" {
				return fmt.Errorf("SchemaRefAdapter.adaptStruct failed to resolve fieldTypeName for %#v", field)
			}

			if !s.addSchemaRef(fieldTypeName, ref) {
				return fmt.Errorf("schemaRefAdapter.adaptStruct unable to add schemaRef for field type " + fieldTypeName)
			}

			schema.WithPropertyRef(field.Name(), ref)
		}
	}

	return nil
}

// getTypesObject ensures one and only one instance of a typesObject is used during processing.
func (s *schemaRefAdapter) getTypesObject(objPtr *types.Object) *types.Object {
	obj := *objPtr
	if obj == nil {
		return nil
	}

	switch elemType := obj.Type().(type) {
	case *types.Named:
		var indirectObj types.Object
		indirectObj = elemType.Obj()
		return &indirectObj
	case *types.Pointer:
		switch pointerType := elemType.Elem().(type) {
		case *types.Named:
			var indirectObj types.Object
			indirectObj = pointerType.Obj()
			return &indirectObj
		default:
			s.analyzer.Logger(fmt.Sprintf("SchemaRefAdapter.GetTypeInfo unhandled type %#v", pointerType))
			return nil
		}
	default:
		s.analyzer.Logger("schemaRefAdapter.getTypesObject unhandled type", elemType)
	}

	s.typeObjectsMutex.RLock()
	typeObject, exists := s.typeObjects[obj.Name()]
	s.typeObjectsMutex.RUnlock()
	if exists {
		return typeObject
	}

	// Publish
	s.typeObjectsMutex.Lock()
	s.typeObjects[obj.Name()] = objPtr
	s.typeObjectsMutex.Unlock()

	return objPtr
}

// getTypesObject ensures one and only one instance of a typesObject is used during processing.
func (s *schemaRefAdapter) getType(typ types.Type) *types.Type {
	s.typesMutex.RLock()
	exists, ok := s.types[typ.String()]
	s.typesMutex.RUnlock()
	if ok {
		return exists
	}

	// Publish
	s.typesMutex.Lock()
	s.types[typ.String()] = &typ
	s.typesMutex.Unlock()

	return &typ
}

func (s *schemaRefAdapter) getSchemaName(typ types.Type) string {
	schemaName := typ.String()
	if strings.Contains(typ.String(), "map") {
		return "object"
	}

	suffix := ""
	// Figure out if this is an array - TODO: may not want to do this actually.
	if strings.HasPrefix(typ.String(), "[]") {
		return "array"
	}

	replace := func(r string) string {
		return strings.Replace(strings.Replace(r, "*", "", -1), "[]", "", -1)
	}

	segments := strings.Split(typ.String(), ".")
	// Handle basic types
	if len(segments) < 2 {
		// s.analyzer.Logger("primitive", typ.String())
		if strings.Contains(typ.String(), "byte") {
			schemaName = "string"
		} else if typ.String() == "interface{}" || strings.Contains(typ.String(), "map") {
			schemaName = "object"
		} else if strings.Contains(typ.String(), "int") {
			schemaName = "integer"
		} else if strings.Contains(typ.String(), "float") || strings.Contains(typ.String(), "double") {
			schemaName = "number"
		} else if strings.Contains(typ.String(), "bool") {
			schemaName = "boolean"
		}
		return replace(schemaName) + suffix
	}

	// if not a basic type, strip * and [] characters
	// s.analyzer.Logger("segmented", schemaName)
	schemaName = replace(segments[len(segments)-1]) + suffix
	// s.analyzer.Logger("clean", schemaName)
	return schemaName
}

func (s *schemaRefAdapter) generateWithoutSaving(parents []*types.Type, typ types.Type) (*openapi3.SchemaRef, error) {
	typeInfo := s.getType(typ)

	for _, parent := range parents {
		if parent == typeInfo {
			return nil, &openapi3gen.CycleError{}
		}
	}

	if cap(parents) == 0 {
		parents = make([]*types.Type, 0, 4)
	}
	parents = append(parents, typeInfo)

	switch pointerType := typ.(type) {
	case *types.Pointer:
		typ = pointerType.Elem()
	}

	schema := &openapi3.Schema{}

	switch typType := typ.(type) {
	case *types.Basic:
		s.adaptBasic(typType, schema)
	case *types.Slice:
		schema.Type = "array"
		items, err := s.generateWithoutSaving(parents, typType.Elem())
		if err != nil {
			return nil, err
		}
		if items != nil {
			if _, ok := s.SchemaRefs[s.getSchemaName(typType.Elem())]; !ok {
				s.SchemaRefs[s.getSchemaName(typType.Elem())] = items
			}
			schema.Items = items
		}
	case *types.Map:
		schema.Type = "object"
		additionalProperties, err := s.generateWithoutSaving(parents, typType.Elem())
		if err != nil {
			return nil, err
		}
		if additionalProperties != nil {
			if _, ok := s.SchemaRefs[s.getSchemaName(typType.Elem())]; !ok {
				s.SchemaRefs[s.getSchemaName(typType.Elem())] = additionalProperties
			}
			schema.AdditionalProperties = additionalProperties
		}
	case *types.Pointer:
		ref, err := s.generateWithoutSaving(parents, typType.Elem())
		if err != nil {
			return nil, err
		}
		if ref != nil {
			if _, ok := s.SchemaRefs[s.getSchemaName(typType)]; !ok {
				s.SchemaRefs[s.getSchemaName(typType)] = ref
			}
		}
	case *types.Named:
		if typType.Obj().Pkg().Name() == "time" {
			return nil, nil
		}
		switch underlying := typType.Underlying().(type) {
		case *types.Struct:
			for i := 0; i < underlying.NumFields(); i++ {
				fieldInfo := underlying.Field(i)
				if fieldInfo.Name() == "TaskStates" {
					s.analyzer.Logger("Got here")
				}
				ref, err := s.generateWithoutSaving(parents, fieldInfo.Type())
				if err != nil {
					return nil, err
				}
				if ref != nil {
					// Add referenced schema if not added already
					if _, ok := s.SchemaRefs[s.getSchemaName(fieldInfo.Type())]; !ok {
						s.SchemaRefs[s.getSchemaName(fieldInfo.Type())] = ref
					}
					if s.isBasic(ref.Value.Type) {
						schema.WithPropertyRef(fieldInfo.Name(), ref)
					} else if ref.Value.Type == "array" {
						targetType := fieldInfo.Type()
						switch sliceType := fieldInfo.Type().(type) {
						case *types.Slice:
							targetType = sliceType.Elem()
						}
						propertyRef := openapi3.NewSchemaRef(fmt.Sprintf("#/components/schemas/%s", s.getSchemaName(targetType)), ref.Value)
						if s.isBasic(ref.Value.Items.Value.Type) {
							propertyRef.Ref = ""
						}
						schema.WithPropertyRef(fieldInfo.Name(), propertyRef)
					} else if ref.Value.Type == "object" {
						targetType := fieldInfo.Type()
						switch mapType := fieldInfo.Type().(type) {
						case *types.Map:
							targetType = mapType.Elem()
						}
						propertyRef := openapi3.NewSchemaRef(fmt.Sprintf("#/components/schemas/%s", s.getSchemaName(targetType)), ref.Value)
						if s.isBasic(s.getSchemaName(targetType)) {
							propertyRef.Ref = ""
						}
						schema.WithPropertyRef(fieldInfo.Name(), ref)
					} else {
						propertyRef := openapi3.NewSchemaRef(fmt.Sprintf("#/components/schemas/%s", s.getSchemaName(fieldInfo.Type())), ref.Value)
						schema.WithPropertyRef(fieldInfo.Name(), propertyRef)
					}
				}
			}
		default:
			// If it's not a struct, then recurse
			ref, err := s.generateWithoutSaving(parents, underlying)
			if err != nil {
				return nil, err
			}
			schema = ref.Value
		}

		// Object only if it has properties
		if schema.Properties != nil {
			schema.Type = "object"
		}
	}

	ref := openapi3.NewSchemaRef("", schema)
	if _, ok := s.SchemaRefs[s.getSchemaName(typ)]; !ok {
		s.SchemaRefs[s.getSchemaName(typ)] = ref
	}
	return ref, nil
}

func (s *schemaRefAdapter) isBasic(typ string) bool {
	if typ == "string" || typ == "boolean" || typ == "number" || typ == "integer" {
		return true
	}

	return false
}

var (
	zeroInt   = float64(0)
	maxInt8   = float64(math.MaxInt8)
	minInt8   = float64(math.MinInt8)
	maxInt16  = float64(math.MaxInt16)
	minInt16  = float64(math.MinInt16)
	maxUint8  = float64(math.MaxUint8)
	maxUint16 = float64(math.MaxUint16)
	maxUint32 = float64(math.MaxUint32)
	maxUint64 = float64(math.MaxUint64)
)
