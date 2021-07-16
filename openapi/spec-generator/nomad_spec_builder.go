package main

import (
	"golang.org/x/tools/go/packages"
)

// newNomadSpecBuilder is a factory method for the NomadSpecBuilder struct
func newNomadSpecBuilder(analyzer *Analyzer, visitor *PackageVisitor) *SpecBuilder {
	builder := &SpecBuilder{
		Visitor: visitor,
	}

	ext := &NomadSpecBuilderExtImpl{
		builder:  builder,
		analyzer: &Analyzer{},
	}

	builder.Ext = ext

	return builder
}

// NomadSpecBuilderExt defines the interface of extension methods that are exposed
// to adapter functions
// TODO: I don't know that any of this needs to be injected anymore if it works
// on the common HandlerFuncProfile interface
type NomadSpecBuilderExt interface {
	SpecBuilder() *SpecBuilder
	Analyzer() *Analyzer
	AdaptHTTPHandler(key string, info *handlerFuncAdapter) error
}

// NomadSpecBuilderExtImpl implements the NomadSpecBuilderExt interface consumed
// by adapter methods
type NomadSpecBuilderExtImpl struct {
	builder  *SpecBuilder
	analyzer *Analyzer
}

// SpecBuilder satisfies the SpecBuilder() method required by the SpecBuilderExt interface
func (e *NomadSpecBuilderExtImpl) SpecBuilder() *SpecBuilder {
	return e.builder
}

// Analyzer satisfies the analyzer() method required by the NomadSpecBuilderExt interface
func (e *NomadSpecBuilderExtImpl) Analyzer() *Analyzer {
	return e.analyzer
}

// AdaptHTTPHandler analyzes the source code for an HTTP Handler and builds an
// Path/PathItem.
func (e *NomadSpecBuilderExtImpl) AdaptHTTPHandler(key string, adapter *handlerFuncAdapter) error {
	//path, err := TypesInfo.GetPath()
	//if err != nil {
	//	return fmt.Errorf("NomadSpecBuilderExtImpl.AdaptHTTPHandler.analyzer.GetPath: %v\n", err)
	//}
	//
	//responseModel, err := e.analyzer.GetResponseModel(httpHandler, result)
	//if err != nil {
	//	return fmt.Errorf("NomadSpecBuilderExtImpl.AdaptHTTPHandler.analyzer.GetPath: %v\n", err)
	//}
	//
	//fmt.Println(responseModel)
	//
	////err = e.addSchema(result, httpHandler, responseModel)
	////if err != nil {
	////	return err
	////}
	//
	//methods, err := e.analyzer.GetMethods(key, httpHandler, result)
	//if err != nil {
	//	return fmt.Errorf("NomadSpecBuilderExtImpl.AdaptHTTPHandler.analyzer.GetMethods: %v\n", err)
	//}
	//
	//for _, method := range methods {
	//	err = e.addOperation(result, key, httpHandler, path, method)
	//	if err != nil {
	//		return err
	//	}
	//}
	return nil
}

//func (e *NomadSpecBuilderExtImpl) addSchema(result *ParseResult, httpHandler *types.Func, schemaName string) error {
//	schemaRef, err := e.toSchemaRef(result, httpHandler, schemaName)
//	if err != nil {
//		return err
//	}
//	e.builder.spec.Model.Components.Schemas[schemaName] = schemaRef
//	return nil
//}
//
//// addOperation adds an operation to a Path.PathItem for a specific method
//// TODO: Simplify this signature
//func (e *NomadSpecBuilderExtImpl) addOperation(result *ParseResult, key string, httpHandler *types.Func, path string, method string) error {
//	operation := &openapi3.Operation{}
//	params, err := e.analyzer.GetParameters(key, httpHandler, result)
//	if err != nil {
//		return fmt.Errorf("NomadSpecBuilderExtImpl.addOperation.analyzer.GetParameters: %v\n", err)
//	}
//
//	for name, param := range params {
//		if _, ok := e.SpecBuilder().spec.Model.Components.Parameters[name]; !ok {
//			paramRef, err := e.toParamRef(param)
//			if err != nil {
//				return err
//			}
//			e.SpecBuilder().spec.Model.Components.Parameters[name] = paramRef
//		}
//		operation.AddParameter(e.SpecBuilder().spec.Model.Components.Parameters[name].Value)
//	}
//	e.SpecBuilder().spec.Model.AddOperation(path, method, operation)
//	return nil
//}
//
//// toParamRef is responsible for adapting source code into a kin-openapi ParamRef
//// TODO: Determine the correct input argument(s)
//func (e *NomadSpecBuilderExtImpl) toParamRef(param *types.Type) (*openapi3.ParameterRef, error) {
//	ref := &openapi3.ParameterRef{
//		Ref: "unset",
//		Value: &openapi3.Parameter{
//			Name: "unset",
//		},
//	}
//	return ref, nil
//}
//
//// toSchemaRef is responsible for adapting source code into a kin-openapi SchemaRef
//// TODO: Determine the correct input argument(s)
//func (e *NomadSpecBuilderExtImpl) toSchemaRef(result *ParseResult, httpHandler *types.Func, responseModel string) (*openapi3.SchemaRef, error) {
//	ref := &openapi3.SchemaRef{
//		Ref:   responseModel,
//		Value: &openapi3.Schema{},
//	}
//	return ref, nil
//}

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

var nomadPackages = []*PackageConfig{
	{
		Config: packages.Config{
			Dir:  "../../api/",
			Mode: loadMode,
		},
		Pattern: ".",
	},
	{
		Config: packages.Config{
			Dir:  "../../command/agent/",
			Mode: loadMode,
		},
		Pattern: ".",
	},
	//{
	//	Config: packages.Config{
	//		Dir:  "../../client/structs/",
	//		Mode: loadMode,
	//	},
	//	Pattern: ".",
	//	Alias: "cstructs",
	//},
}
