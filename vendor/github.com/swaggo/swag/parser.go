package swag

import (
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/build"
	goparser "go/parser"
	"go/token"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/KyleBanks/depth"
	"github.com/go-openapi/spec"
)

const (
	// CamelCase indicates using CamelCase strategy for struct field.
	CamelCase = "camelcase"

	// PascalCase indicates using PascalCase strategy for struct field.
	PascalCase = "pascalcase"

	// SnakeCase indicates using SnakeCase strategy for struct field.
	SnakeCase = "snakecase"
)

var (
	//ErrRecursiveParseStruct recursively parsing struct
	ErrRecursiveParseStruct = errors.New("recursively parsing struct")

	//ErrFuncTypeField field type is func
	ErrFuncTypeField = errors.New("field type is func")

	// ErrFailedConvertPrimitiveType Failed to convert for swag to interpretable type
	ErrFailedConvertPrimitiveType = errors.New("swag property: failed convert primitive type")
)

// Parser implements a parser for Go source files.
type Parser struct {
	// swagger represents the root document object for the API specification
	swagger *spec.Swagger

	//packages store entities of APIs, definitions, file, package path etc.  and their relations
	packages *PackagesDefinitions

	//parsedSchemas store schemas which have been parsed from ast.TypeSpec
	parsedSchemas map[*TypeSpecDef]*Schema

	//outputSchemas store schemas which will be export to swagger
	outputSchemas map[*TypeSpecDef]*Schema

	//existSchemaNames store names of models for conflict determination
	existSchemaNames map[string]*Schema

	//toBeRenamedSchemas names of models to be renamed
	toBeRenamedSchemas map[string]string

	//toBeRenamedSchemas URLs of ref models to be renamed
	toBeRenamedRefURLs []*url.URL

	PropNamingStrategy string

	ParseVendor bool

	// ParseDependencies whether swag should be parse outside dependency folder
	ParseDependency bool

	// ParseInternal whether swag should parse internal packages
	ParseInternal bool

	// structStack stores full names of the structures that were already parsed or are being parsed now
	structStack []*TypeSpecDef

	// markdownFileDir holds the path to the folder, where markdown files are stored
	markdownFileDir string

	// codeExampleFilesDir holds path to the folder, where code example files are stored
	codeExampleFilesDir string

	// collectionFormatInQuery set the default collectionFormat otherwise then 'csv' for array in query params
	collectionFormatInQuery string

	// excludes excludes dirs and files in SearchDir
	excludes map[string]bool
}

// New creates a new Parser with default properties.
func New(options ...func(*Parser)) *Parser {
	parser := &Parser{
		swagger: &spec.Swagger{
			SwaggerProps: spec.SwaggerProps{
				Info: &spec.Info{
					InfoProps: spec.InfoProps{
						Contact: &spec.ContactInfo{},
						License: nil,
					},
					VendorExtensible: spec.VendorExtensible{
						Extensions: spec.Extensions{},
					},
				},
				Paths: &spec.Paths{
					Paths: make(map[string]spec.PathItem),
				},
				Definitions: make(map[string]spec.Schema),
			},
		},
		packages:           NewPackagesDefinitions(),
		parsedSchemas:      make(map[*TypeSpecDef]*Schema),
		outputSchemas:      make(map[*TypeSpecDef]*Schema),
		existSchemaNames:   make(map[string]*Schema),
		toBeRenamedSchemas: make(map[string]string),
		excludes:           make(map[string]bool),
	}

	for _, option := range options {
		option(parser)
	}

	return parser
}

// SetMarkdownFileDirectory sets the directory to search for markdownfiles
func SetMarkdownFileDirectory(directoryPath string) func(*Parser) {
	return func(p *Parser) {
		p.markdownFileDir = directoryPath
	}
}

// SetCodeExamplesDirectory sets the directory to search for code example files
func SetCodeExamplesDirectory(directoryPath string) func(*Parser) {
	return func(p *Parser) {
		p.codeExampleFilesDir = directoryPath
	}
}

// SetExcludedDirsAndFiles sets directories and files to be excluded when searching
func SetExcludedDirsAndFiles(excludes string) func(*Parser) {
	return func(p *Parser) {
		for _, f := range strings.Split(excludes, ",") {
			f = strings.TrimSpace(f)
			if f != "" {
				f = filepath.Clean(f)
				p.excludes[f] = true
			}
		}
	}
}

// ParseAPI parses general api info for given searchDir and mainAPIFile
func (parser *Parser) ParseAPI(searchDir, mainAPIFile string, parseDepth int) error {
	Printf("Generate general API Info, search dir:%s", searchDir)

	packageDir, err := getPkgName(searchDir)
	if err != nil {
		Printf("warning: failed to get package name in dir: %s, error: %s", searchDir, err.Error())
	}

	if err = parser.getAllGoFileInfo(packageDir, searchDir); err != nil {
		return err
	}

	absMainAPIFilePath, err := filepath.Abs(filepath.Join(searchDir, mainAPIFile))
	if err != nil {
		return err
	}

	if parser.ParseDependency {
		var t depth.Tree
		t.ResolveInternal = true
		t.MaxDepth = parseDepth

		pkgName, err := getPkgName(filepath.Dir(absMainAPIFilePath))
		if err != nil {
			return err
		}
		if err := t.Resolve(pkgName); err != nil {
			return fmt.Errorf("pkg %s cannot find all dependencies, %s", pkgName, err)
		}
		for i := 0; i < len(t.Root.Deps); i++ {
			if err := parser.getAllGoFileInfoFromDeps(&t.Root.Deps[i]); err != nil {
				return err
			}
		}
	}

	if err = parser.ParseGeneralAPIInfo(absMainAPIFilePath); err != nil {
		return err
	}

	parser.parsedSchemas, err = parser.packages.ParseTypes()
	if err != nil {
		return err
	}

	if err = parser.packages.RangeFiles(parser.ParseRouterAPIInfo); err != nil {
		return err
	}

	parser.renameRefSchemas()

	return parser.checkOperationIDUniqueness()
}

func getPkgName(searchDir string) (string, error) {
	cmd := exec.Command("go", "list", "-f={{.ImportPath}}")
	cmd.Dir = searchDir
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("execute go list command, %s, stdout:%s, stderr:%s", err, stdout.String(), stderr.String())
	}

	outStr, _ := stdout.String(), stderr.String()

	if outStr[0] == '_' { // will shown like _/{GOPATH}/src/{YOUR_PACKAGE} when NOT enable GO MODULE.
		outStr = strings.TrimPrefix(outStr, "_"+build.Default.GOPATH+"/src/")
	}
	f := strings.Split(outStr, "\n")
	outStr = f[0]

	return outStr, nil
}

func initIfEmpty(license *spec.License) *spec.License {
	if license == nil {
		return new(spec.License)
	}

	return license
}

// ParseGeneralAPIInfo parses general api info for given mainAPIFile path
func (parser *Parser) ParseGeneralAPIInfo(mainAPIFile string) error {
	fileSet := token.NewFileSet()
	fileTree, err := goparser.ParseFile(fileSet, mainAPIFile, nil, goparser.ParseComments)
	if err != nil {
		return fmt.Errorf("cannot parse source files %s: %s", mainAPIFile, err)
	}

	parser.swagger.Swagger = "2.0"
	securityMap := map[string]*spec.SecurityScheme{}

	for _, comment := range fileTree.Comments {
		if !isGeneralAPIComment(comment) {
			continue
		}
		comments := strings.Split(comment.Text(), "\n")
		previousAttribute := ""
		// parsing classic meta data model
		for i, commentLine := range comments {
			attribute := strings.ToLower(strings.Split(commentLine, " ")[0])
			value := strings.TrimSpace(commentLine[len(attribute):])
			multilineBlock := false
			if previousAttribute == attribute {
				multilineBlock = true
			}
			switch attribute {
			case "@version":
				parser.swagger.Info.Version = value
			case "@title":
				parser.swagger.Info.Title = value
			case "@description":
				if multilineBlock {
					parser.swagger.Info.Description += "\n" + value
					continue
				}
				parser.swagger.Info.Description = value
			case "@description.markdown":
				commentInfo, err := getMarkdownForTag("api", parser.markdownFileDir)
				if err != nil {
					return err
				}
				parser.swagger.Info.Description = string(commentInfo)
			case "@termsofservice":
				parser.swagger.Info.TermsOfService = value
			case "@contact.name":
				parser.swagger.Info.Contact.Name = value
			case "@contact.email":
				parser.swagger.Info.Contact.Email = value
			case "@contact.url":
				parser.swagger.Info.Contact.URL = value
			case "@license.name":
				parser.swagger.Info.License = initIfEmpty(parser.swagger.Info.License)
				parser.swagger.Info.License.Name = value
			case "@license.url":
				parser.swagger.Info.License = initIfEmpty(parser.swagger.Info.License)
				parser.swagger.Info.License.URL = value
			case "@host":
				parser.swagger.Host = value
			case "@basepath":
				parser.swagger.BasePath = value
			case "@schemes":
				parser.swagger.Schemes = getSchemes(commentLine)
			case "@tag.name":
				parser.swagger.Tags = append(parser.swagger.Tags, spec.Tag{
					TagProps: spec.TagProps{
						Name: value,
					},
				})
			case "@tag.description":
				tag := parser.swagger.Tags[len(parser.swagger.Tags)-1]
				tag.TagProps.Description = value
				replaceLastTag(parser.swagger.Tags, tag)
			case "@tag.description.markdown":
				tag := parser.swagger.Tags[len(parser.swagger.Tags)-1]
				commentInfo, err := getMarkdownForTag(tag.TagProps.Name, parser.markdownFileDir)
				if err != nil {
					return err
				}
				tag.TagProps.Description = string(commentInfo)
				replaceLastTag(parser.swagger.Tags, tag)
			case "@tag.docs.url":
				tag := parser.swagger.Tags[len(parser.swagger.Tags)-1]
				tag.TagProps.ExternalDocs = &spec.ExternalDocumentation{
					URL: value,
				}
				replaceLastTag(parser.swagger.Tags, tag)
			case "@tag.docs.description":
				tag := parser.swagger.Tags[len(parser.swagger.Tags)-1]
				if tag.TagProps.ExternalDocs == nil {
					return fmt.Errorf("%s needs to come after a @tags.docs.url", attribute)
				}
				tag.TagProps.ExternalDocs.Description = value
				replaceLastTag(parser.swagger.Tags, tag)
			case "@securitydefinitions.basic":
				securityMap[value] = spec.BasicAuth()
			case "@securitydefinitions.apikey":
				attrMap, _, _, err := extractSecurityAttribute(attribute, []string{"@in", "@name"}, comments[i+1:])
				if err != nil {
					return err
				}
				securityMap[value] = spec.APIKeyAuth(attrMap["@name"], attrMap["@in"])
			case "@securitydefinitions.oauth2.application":
				attrMap, scopes, extensions, err := extractSecurityAttribute(attribute, []string{"@tokenurl"}, comments[i+1:])
				if err != nil {
					return err
				}
				securityMap[value] = securitySchemeOAuth2Application(attrMap["@tokenurl"], scopes, extensions)
			case "@securitydefinitions.oauth2.implicit":
				attrMap, scopes, extensions, err := extractSecurityAttribute(attribute, []string{"@authorizationurl"}, comments[i+1:])
				if err != nil {
					return err
				}
				securityMap[value] = securitySchemeOAuth2Implicit(attrMap["@authorizationurl"], scopes, extensions)
			case "@securitydefinitions.oauth2.password":
				attrMap, scopes, extensions, err := extractSecurityAttribute(attribute, []string{"@tokenurl"}, comments[i+1:])
				if err != nil {
					return err
				}
				securityMap[value] = securitySchemeOAuth2Password(attrMap["@tokenurl"], scopes, extensions)
			case "@securitydefinitions.oauth2.accesscode":
				attrMap, scopes, extensions, err := extractSecurityAttribute(attribute, []string{"@tokenurl", "@authorizationurl"}, comments[i+1:])
				if err != nil {
					return err
				}
				securityMap[value] = securitySchemeOAuth2AccessToken(attrMap["@authorizationurl"], attrMap["@tokenurl"], scopes, extensions)
			case "@x-tokenname":
				// ignore this
				break
			case "@query.collection.format":
				parser.collectionFormatInQuery = value
			default:
				prefixExtension := "@x-"
				if len(attribute) > 5 { // Prefix extension + 1 char + 1 space  + 1 char
					if attribute[:len(prefixExtension)] == prefixExtension {
						var valueJSON interface{}
						split := strings.SplitAfter(commentLine, attribute+" ")
						if len(split) < 2 {
							return fmt.Errorf("annotation %s need a value", attribute)
						}
						extensionName := "x-" + strings.SplitAfter(attribute, prefixExtension)[1]
						if err := json.Unmarshal([]byte(split[1]), &valueJSON); err != nil {
							return fmt.Errorf("annotation %s need a valid json value", attribute)
						}

						if strings.Contains(extensionName, "logo") {
							parser.swagger.Info.Extensions.Add(extensionName, valueJSON)
						} else {
							parser.swagger.AddExtension(extensionName, valueJSON)
						}
					}
				}
			}
			previousAttribute = attribute
		}
	}

	if len(securityMap) > 0 {
		parser.swagger.SecurityDefinitions = securityMap
	}

	return nil
}

func isGeneralAPIComment(comment *ast.CommentGroup) bool {
	for _, commentLine := range strings.Split(comment.Text(), "\n") {
		attribute := strings.ToLower(strings.Split(commentLine, " ")[0])
		switch attribute {
		// The @summary, @router, @success,@failure  annotation belongs to Operation
		case "@summary", "@router", "@success", "@failure", "@response":
			return false
		}
	}
	return true
}

func extractSecurityAttribute(context string, search []string, lines []string) (map[string]string, map[string]string, map[string]interface{}, error) {
	attrMap := map[string]string{}
	scopes := map[string]string{}
	extensions := map[string]interface{}{}
	for _, v := range lines {
		securityAttr := strings.ToLower(strings.Split(v, " ")[0])
		for _, findterm := range search {
			if securityAttr == findterm {
				attrMap[securityAttr] = strings.TrimSpace(v[len(securityAttr):])
				continue
			}
		}
		isExists, err := isExistsScope(securityAttr)
		if err != nil {
			return nil, nil, nil, err
		}
		if isExists {
			scopScheme, err := getScopeScheme(securityAttr)
			if err != nil {
				return nil, nil, nil, err
			}
			scopes[scopScheme] = v[len(securityAttr):]
		}
		if securityAttr == "@x-tokenname" {
			extensions["x-tokenName"] = strings.TrimSpace(v[len(securityAttr):])
		}
		// next securityDefinitions
		if strings.Index(securityAttr, "@securitydefinitions.") == 0 {
			break
		}
	}
	if len(attrMap) != len(search) {
		return nil, nil, nil, fmt.Errorf("%s is %v required", context, search)
	}
	return attrMap, scopes, extensions, nil
}

func securitySchemeOAuth2Application(tokenurl string, scopes map[string]string, extensions map[string]interface{}) *spec.SecurityScheme {
	securityScheme := spec.OAuth2Application(tokenurl)
	securityScheme.VendorExtensible.Extensions = handleSecuritySchemaExtensions(extensions)
	for scope, description := range scopes {
		securityScheme.AddScope(scope, description)
	}
	return securityScheme
}

func securitySchemeOAuth2Implicit(authorizationurl string, scopes map[string]string, extensions map[string]interface{}) *spec.SecurityScheme {
	securityScheme := spec.OAuth2Implicit(authorizationurl)
	securityScheme.VendorExtensible.Extensions = handleSecuritySchemaExtensions(extensions)
	for scope, description := range scopes {
		securityScheme.AddScope(scope, description)
	}
	return securityScheme
}

func securitySchemeOAuth2Password(tokenurl string, scopes map[string]string, extensions map[string]interface{}) *spec.SecurityScheme {
	securityScheme := spec.OAuth2Password(tokenurl)
	securityScheme.VendorExtensible.Extensions = handleSecuritySchemaExtensions(extensions)
	for scope, description := range scopes {
		securityScheme.AddScope(scope, description)
	}
	return securityScheme
}

func securitySchemeOAuth2AccessToken(authorizationurl, tokenurl string, scopes map[string]string, extensions map[string]interface{}) *spec.SecurityScheme {
	securityScheme := spec.OAuth2AccessToken(authorizationurl, tokenurl)
	securityScheme.VendorExtensible.Extensions = handleSecuritySchemaExtensions(extensions)
	for scope, description := range scopes {
		securityScheme.AddScope(scope, description)
	}
	return securityScheme
}

func handleSecuritySchemaExtensions(providedExtensions map[string]interface{}) spec.Extensions {
	var extensions spec.Extensions
	if len(providedExtensions) > 0 {
		extensions = make(map[string]interface{}, len(providedExtensions))
		for key, value := range providedExtensions {
			extensions[key] = value
		}
	}
	return extensions
}

func getMarkdownForTag(tagName string, dirPath string) ([]byte, error) {
	filesInfos, err := ioutil.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	for _, fileInfo := range filesInfos {
		if fileInfo.IsDir() {
			continue
		}
		fileName := fileInfo.Name()

		if !strings.Contains(fileName, ".md") {
			continue
		}

		if strings.Contains(fileName, tagName) {
			fullPath := filepath.Join(dirPath, fileName)
			commentInfo, err := ioutil.ReadFile(fullPath)
			if err != nil {
				return nil, fmt.Errorf("Failed to read markdown file %s error: %s ", fullPath, err)
			}
			return commentInfo, nil
		}
	}
	return nil, fmt.Errorf("Unable to find markdown file for tag %s in the given directory", tagName)
}

func getScopeScheme(scope string) (string, error) {
	scopeValue := scope[strings.Index(scope, "@scope."):]
	if scopeValue == "" {
		return "", fmt.Errorf("@scope is empty")
	}
	return scope[len("@scope."):], nil
}

func isExistsScope(scope string) (bool, error) {
	s := strings.Fields(scope)
	for _, v := range s {
		if strings.Contains(v, "@scope.") {
			if strings.Contains(v, ",") {
				return false, fmt.Errorf("@scope can't use comma(,) get=" + v)
			}
		}
	}
	return strings.Contains(scope, "@scope."), nil
}

// getSchemes parses swagger schemes for given commentLine
func getSchemes(commentLine string) []string {
	attribute := strings.ToLower(strings.Split(commentLine, " ")[0])
	return strings.Split(strings.TrimSpace(commentLine[len(attribute):]), " ")
}

// ParseRouterAPIInfo parses router api info for given astFile
func (parser *Parser) ParseRouterAPIInfo(fileName string, astFile *ast.File) error {
	for _, astDescription := range astFile.Decls {
		switch astDeclaration := astDescription.(type) {
		case *ast.FuncDecl:
			if astDeclaration.Doc != nil && astDeclaration.Doc.List != nil {
				operation := NewOperation(parser, SetCodeExampleFilesDirectory(parser.codeExampleFilesDir)) //for per 'function' comment, create a new 'Operation' object
				for _, comment := range astDeclaration.Doc.List {
					if err := operation.ParseComment(comment.Text, astFile); err != nil {
						return fmt.Errorf("ParseComment error in file %s :%+v", fileName, err)
					}
				}
				var pathItem spec.PathItem
				var ok bool

				if pathItem, ok = parser.swagger.Paths.Paths[operation.Path]; !ok {
					pathItem = spec.PathItem{}
				}
				switch strings.ToUpper(operation.HTTPMethod) {
				case http.MethodGet:
					pathItem.Get = &operation.Operation
				case http.MethodPost:
					pathItem.Post = &operation.Operation
				case http.MethodDelete:
					pathItem.Delete = &operation.Operation
				case http.MethodPut:
					pathItem.Put = &operation.Operation
				case http.MethodPatch:
					pathItem.Patch = &operation.Operation
				case http.MethodHead:
					pathItem.Head = &operation.Operation
				case http.MethodOptions:
					pathItem.Options = &operation.Operation
				}

				parser.swagger.Paths.Paths[operation.Path] = pathItem
			}
		}
	}

	return nil
}

func convertFromSpecificToPrimitive(typeName string) (string, error) {
	name := typeName
	if strings.ContainsRune(name, '.') {
		name = strings.Split(name, ".")[1]
	}
	switch strings.ToUpper(name) {
	case "TIME", "OBJECTID", "UUID":
		return STRING, nil
	case "DECIMAL":
		return NUMBER, nil
	}
	return typeName, ErrFailedConvertPrimitiveType
}

func (parser *Parser) getTypeSchema(typeName string, file *ast.File, ref bool) (*spec.Schema, error) {
	if IsGolangPrimitiveType(typeName) {
		return PrimitiveSchema(TransToValidSchemeType(typeName)), nil
	}

	if schemaType, err := convertFromSpecificToPrimitive(typeName); err == nil {
		return PrimitiveSchema(schemaType), nil
	}

	typeSpecDef := parser.packages.FindTypeSpec(typeName, file)
	if typeSpecDef == nil {
		return nil, fmt.Errorf("cannot find type definition: %s", typeName)
	}

	schema, ok := parser.parsedSchemas[typeSpecDef]
	if !ok {
		var err error
		schema, err = parser.ParseDefinition(typeSpecDef)
		if err == ErrRecursiveParseStruct {
			if ref {
				return parser.getRefTypeSchema(typeSpecDef, schema), nil
			}

		} else if err != nil {
			return nil, err
		}
	}

	if ref && len(schema.Schema.Type) > 0 && schema.Schema.Type[0] == OBJECT {
		return parser.getRefTypeSchema(typeSpecDef, schema), nil
	}
	return schema.Schema, nil
}

func (parser *Parser) renameRefSchemas() {
	if len(parser.toBeRenamedSchemas) == 0 {
		return
	}

	//rename schemas in swagger.Definitions
	for name, pkgPath := range parser.toBeRenamedSchemas {
		if schema, ok := parser.swagger.Definitions[name]; ok {
			delete(parser.swagger.Definitions, name)
			name = parser.renameSchema(name, pkgPath)
			parser.swagger.Definitions[name] = schema
		}
	}

	//rename URLs if match
	for _, url := range parser.toBeRenamedRefURLs {
		parts := strings.Split(url.Fragment, "/")
		name := parts[len(parts)-1]
		if pkgPath, ok := parser.toBeRenamedSchemas[name]; ok {
			parts[len(parts)-1] = parser.renameSchema(name, pkgPath)
			url.Fragment = strings.Join(parts, "/")
		}
	}
}

func (parser *Parser) renameSchema(name, pkgPath string) string {
	parts := strings.Split(name, ".")
	name = fullTypeName(pkgPath, parts[len(parts)-1])
	name = strings.ReplaceAll(name, "/", "_")
	return name
}

func (parser *Parser) getRefTypeSchema(typeSpecDef *TypeSpecDef, schema *Schema) *spec.Schema {
	if _, ok := parser.outputSchemas[typeSpecDef]; !ok {
		if existSchema, ok := parser.existSchemaNames[schema.Name]; ok {
			//store the first one to be renamed after parsing over
			if _, ok = parser.toBeRenamedSchemas[existSchema.Name]; !ok {
				parser.toBeRenamedSchemas[existSchema.Name] = existSchema.PkgPath
			}
			//rename not the first one
			schema.Name = parser.renameSchema(schema.Name, schema.PkgPath)
		} else {
			parser.existSchemaNames[schema.Name] = schema
		}
		if schema.Schema != nil {
			parser.swagger.Definitions[schema.Name] = *schema.Schema
		} else {
			parser.swagger.Definitions[schema.Name] = spec.Schema{}
		}
		parser.outputSchemas[typeSpecDef] = schema
	}

	refSchema := RefSchema(schema.Name)
	//store every URL
	parser.toBeRenamedRefURLs = append(parser.toBeRenamedRefURLs, refSchema.Ref.Ref.GetURL())
	return refSchema
}

func (parser *Parser) isInStructStack(typeSpecDef *TypeSpecDef) bool {
	for _, specDef := range parser.structStack {
		if typeSpecDef == specDef {
			return true
		}
	}
	return false
}

// ParseDefinition parses given type spec that corresponds to the type under
// given name and package, and populates swagger schema definitions registry
// with a schema for the given type
func (parser *Parser) ParseDefinition(typeSpecDef *TypeSpecDef) (*Schema, error) {
	typeName := typeSpecDef.FullName()
	refTypeName := TypeDocName(typeName, typeSpecDef.TypeSpec)

	if schema, ok := parser.parsedSchemas[typeSpecDef]; ok {
		Println("Skipping '" + typeName + "', already parsed.")
		return schema, nil
	}

	if parser.isInStructStack(typeSpecDef) {
		Println("Skipping '" + typeName + "', recursion detected.")
		return &Schema{
				Name:    refTypeName,
				PkgPath: typeSpecDef.PkgPath,
				Schema:  PrimitiveSchema(OBJECT)},
			ErrRecursiveParseStruct
	}
	parser.structStack = append(parser.structStack, typeSpecDef)

	Println("Generating " + typeName)

	schema, err := parser.parseTypeExpr(typeSpecDef.File, typeSpecDef.TypeSpec.Type, false)
	if err != nil {
		return nil, err
	}
	s := &Schema{Name: refTypeName, PkgPath: typeSpecDef.PkgPath, Schema: schema}
	parser.parsedSchemas[typeSpecDef] = s

	//update an empty schema as a result of recursion
	if s2, ok := parser.outputSchemas[typeSpecDef]; ok {
		parser.swagger.Definitions[s2.Name] = *schema
	}

	return s, nil
}

func fullTypeName(pkgName, typeName string) string {
	if pkgName != "" {
		return pkgName + "." + typeName
	}
	return typeName
}

// parseTypeExpr parses given type expression that corresponds to the type under
// given name and package, and returns swagger schema for it.
func (parser *Parser) parseTypeExpr(file *ast.File, typeExpr ast.Expr, ref bool) (*spec.Schema, error) {
	switch expr := typeExpr.(type) {
	// type Foo struct {...}
	case *ast.StructType:
		return parser.parseStruct(file, expr.Fields)

	// type Foo Baz
	case *ast.Ident:
		return parser.getTypeSchema(expr.Name, file, ref)

	// type Foo *Baz
	case *ast.StarExpr:
		return parser.parseTypeExpr(file, expr.X, ref)

	// type Foo pkg.Bar
	case *ast.SelectorExpr:
		if xIdent, ok := expr.X.(*ast.Ident); ok {
			return parser.getTypeSchema(fullTypeName(xIdent.Name, expr.Sel.Name), file, ref)
		}
	// type Foo []Baz
	case *ast.ArrayType:
		itemSchema, err := parser.parseTypeExpr(file, expr.Elt, true)
		if err != nil {
			return nil, err
		}
		return spec.ArrayProperty(itemSchema), nil
	// type Foo map[string]Bar
	case *ast.MapType:
		if _, ok := expr.Value.(*ast.InterfaceType); ok {
			return spec.MapProperty(nil), nil
		}
		schema, err := parser.parseTypeExpr(file, expr.Value, true)
		if err != nil {
			return nil, err
		}
		return spec.MapProperty(schema), nil
	case *ast.FuncType:
		return nil, ErrFuncTypeField
	// ...
	default:
		Printf("Type definition of type '%T' is not supported yet. Using 'object' instead.\n", typeExpr)
	}

	return PrimitiveSchema(OBJECT), nil
}

func (parser *Parser) parseStruct(file *ast.File, fields *ast.FieldList) (*spec.Schema, error) {

	required := make([]string, 0)
	properties := make(map[string]spec.Schema)
	for _, field := range fields.List {
		fieldProps, requiredFromAnon, err := parser.parseStructField(file, field)
		if err == ErrFuncTypeField {
			continue
		} else if err != nil {
			return nil, err
		} else if len(fieldProps) == 0 {
			continue
		}
		required = append(required, requiredFromAnon...)
		for k, v := range fieldProps {
			properties[k] = v
		}
	}

	sort.Strings(required)

	return &spec.Schema{
		SchemaProps: spec.SchemaProps{
			Type:       []string{OBJECT},
			Properties: properties,
			Required:   required,
		}}, nil
}

type structField struct {
	name         string
	desc         string
	schemaType   string
	arrayType    string
	formatType   string
	isRequired   bool
	readOnly     bool
	crossPkg     string
	exampleValue interface{}
	maximum      *float64
	minimum      *float64
	maxLength    *int64
	minLength    *int64
	enums        []interface{}
	defaultValue interface{}
	extensions   map[string]interface{}
}

func (parser *Parser) parseStructField(file *ast.File, field *ast.Field) (map[string]spec.Schema, []string, error) {
	if field.Names == nil {
		if field.Tag != nil {
			skip, ok := reflect.StructTag(strings.ReplaceAll(field.Tag.Value, "`", "")).Lookup("swaggerignore")
			if ok && strings.EqualFold(skip, "true") {
				return nil, nil, nil
			}
		}

		typeName, err := getFieldType(field.Type)
		if err != nil {
			return nil, nil, err
		}
		schema, err := parser.getTypeSchema(typeName, file, false)
		if err != nil {
			return nil, nil, err
		}
		if len(schema.Type) > 0 && schema.Type[0] == OBJECT {
			if len(schema.Properties) == 0 {
				return nil, nil, nil
			}

			properties := map[string]spec.Schema{}
			for k, v := range schema.Properties {
				properties[k] = v
			}
			return properties, schema.SchemaProps.Required, nil
		}
		//for alias type of non-struct types ,such as array,map, etc. ignore field tag.
		return map[string]spec.Schema{typeName: *schema}, nil, nil
	}

	fieldName, schema, err := parser.getFieldName(field)
	if err != nil {
		return nil, nil, err
	}
	if fieldName == "" {
		return nil, nil, nil
	}
	if schema == nil {
		typeName, err := getFieldType(field.Type)
		if err == nil {
			//named type
			schema, err = parser.getTypeSchema(typeName, file, true)
		} else {
			//unnamed type
			schema, err = parser.parseTypeExpr(file, field.Type, false)
		}
		if err != nil {
			return nil, nil, err
		}
	}

	types := parser.GetSchemaTypePath(schema, 2)
	if len(types) == 0 {
		return nil, nil, fmt.Errorf("invalid type for field: %s", field.Names[0])
	}

	structField, err := parser.parseFieldTag(field, types)
	if err != nil {
		return nil, nil, err
	}

	if structField.schemaType == "string" && types[0] != structField.schemaType {
		schema = PrimitiveSchema(structField.schemaType)
	}

	schema.Description = structField.desc
	schema.ReadOnly = structField.readOnly
	schema.Default = structField.defaultValue
	schema.Example = structField.exampleValue
	schema.Format = structField.formatType
	schema.Extensions = structField.extensions
	eleSchema := schema
	if structField.schemaType == "array" {
		eleSchema = schema.Items.Schema
	}
	eleSchema.Maximum = structField.maximum
	eleSchema.Minimum = structField.minimum
	eleSchema.MaxLength = structField.maxLength
	eleSchema.MinLength = structField.minLength
	eleSchema.Enum = structField.enums

	var tagRequired []string
	if structField.isRequired {
		tagRequired = append(tagRequired, fieldName)
	}
	return map[string]spec.Schema{fieldName: *schema}, tagRequired, nil
}

func getFieldType(field ast.Expr) (string, error) {
	switch ftype := field.(type) {
	case *ast.Ident:
		return ftype.Name, nil
	case *ast.SelectorExpr:
		packageName, err := getFieldType(ftype.X)
		if err != nil {
			return "", err
		}
		return fullTypeName(packageName, ftype.Sel.Name), nil

	case *ast.StarExpr:
		fullName, err := getFieldType(ftype.X)
		if err != nil {
			return "", err
		}
		return fullName, nil
	}
	return "", fmt.Errorf("unknown field type %#v", field)
}

func (parser *Parser) getFieldName(field *ast.Field) (name string, schema *spec.Schema, err error) {
	// Skip non-exported fields.
	if !ast.IsExported(field.Names[0].Name) {
		return "", nil, nil
	}

	if field.Tag != nil {
		// `json:"tag"` -> json:"tag"
		structTag := reflect.StructTag(strings.Replace(field.Tag.Value, "`", "", -1))
		if ignoreTag := structTag.Get("swaggerignore"); strings.EqualFold(ignoreTag, "true") {
			return "", nil, nil
		}

		name = structTag.Get("json")
		// json:"tag,hoge"
		if name = strings.TrimSpace(strings.Split(name, ",")[0]); name == "-" {
			return "", nil, nil
		}

		if typeTag := structTag.Get("swaggertype"); typeTag != "" {
			parts := strings.Split(typeTag, ",")
			schema, err = BuildCustomSchema(parts)
			if err != nil {
				return "", nil, err
			}
		}
	}

	if name == "" {
		switch parser.PropNamingStrategy {
		case SnakeCase:
			name = toSnakeCase(field.Names[0].Name)
		case PascalCase:
			name = field.Names[0].Name
		case CamelCase:
			name = toLowerCamelCase(field.Names[0].Name)
		default:
			name = toLowerCamelCase(field.Names[0].Name)
		}
	}
	return name, schema, err
}

func (parser *Parser) parseFieldTag(field *ast.Field, types []string) (*structField, error) {
	structField := &structField{
		//    name:       field.Names[0].Name,
		schemaType: types[0],
	}
	if len(types) > 1 && (types[0] == "array" || types[0] == "object") {
		structField.arrayType = types[1]
	}

	if field.Doc != nil {
		structField.desc = strings.TrimSpace(field.Doc.Text())
	}
	if structField.desc == "" && field.Comment != nil {
		structField.desc = strings.TrimSpace(field.Comment.Text())
	}

	if field.Tag == nil {
		return structField, nil
	}
	// `json:"tag"` -> json:"tag"
	structTag := reflect.StructTag(strings.Replace(field.Tag.Value, "`", "", -1))

	jsonTag := structTag.Get("json")
	// json:"name,string" or json:",string"
	hasStringTag := strings.Contains(jsonTag, ",string")

	if exampleTag := structTag.Get("example"); exampleTag != "" {
		if hasStringTag {
			// then the example must be in string format
			structField.exampleValue = exampleTag
		} else {
			example, err := defineTypeOfExample(structField.schemaType, structField.arrayType, exampleTag)
			if err != nil {
				return nil, err
			}
			structField.exampleValue = example
		}
	}
	if formatTag := structTag.Get("format"); formatTag != "" {
		structField.formatType = formatTag
	}
	if bindingTag := structTag.Get("binding"); bindingTag != "" {
		for _, val := range strings.Split(bindingTag, ",") {
			if val == "required" {
				structField.isRequired = true
				break
			}
		}
	}
	if validateTag := structTag.Get("validate"); validateTag != "" {
		for _, val := range strings.Split(validateTag, ",") {
			if val == "required" {
				structField.isRequired = true
				break
			}
		}
	}
	if extensionsTag := structTag.Get("extensions"); extensionsTag != "" {
		structField.extensions = map[string]interface{}{}
		for _, val := range strings.Split(extensionsTag, ",") {
			parts := strings.SplitN(val, "=", 2)
			if len(parts) == 2 {
				structField.extensions[parts[0]] = parts[1]
			} else {
				structField.extensions[parts[0]] = true
			}
		}
	}
	if enumsTag := structTag.Get("enums"); enumsTag != "" {
		enumType := structField.schemaType
		if structField.schemaType == ARRAY {
			enumType = structField.arrayType
		}

		for _, e := range strings.Split(enumsTag, ",") {
			value, err := defineType(enumType, e)
			if err != nil {
				return nil, err
			}
			structField.enums = append(structField.enums, value)
		}
	}
	if defaultTag := structTag.Get("default"); defaultTag != "" {
		value, err := defineType(structField.schemaType, defaultTag)
		if err != nil {
			return nil, err
		}
		structField.defaultValue = value
	}

	if IsNumericType(structField.schemaType) || IsNumericType(structField.arrayType) {
		maximum, err := getFloatTag(structTag, "maximum")
		if err != nil {
			return nil, err
		}
		structField.maximum = maximum

		minimum, err := getFloatTag(structTag, "minimum")
		if err != nil {
			return nil, err
		}
		structField.minimum = minimum
	}
	if structField.schemaType == STRING || structField.arrayType == STRING {
		maxLength, err := getIntTag(structTag, "maxLength")
		if err != nil {
			return nil, err
		}
		structField.maxLength = maxLength

		minLength, err := getIntTag(structTag, "minLength")
		if err != nil {
			return nil, err
		}
		structField.minLength = minLength
	}
	if readOnly := structTag.Get("readonly"); readOnly != "" {
		structField.readOnly = readOnly == "true"
	}

	// perform this after setting everything else (min, max, etc...)
	if hasStringTag {

		// @encoding/json: "It applies only to fields of string, floating point, integer, or boolean types."
		defaultValues := map[string]string{
			// Zero Values as string
			STRING:  "",
			INTEGER: "0",
			BOOLEAN: "false",
			NUMBER:  "0",
		}

		if defaultValue, ok := defaultValues[structField.schemaType]; ok {
			structField.schemaType = STRING

			if structField.exampleValue == nil {
				// if exampleValue is not defined by the user,
				// we will force an example with a correct value
				// (eg: int->"0", bool:"false")
				structField.exampleValue = defaultValue
			}
		}
	}

	return structField, nil
}

// GetSchemaTypePath get path of schema type
func (parser *Parser) GetSchemaTypePath(schema *spec.Schema, depth int) []string {
	if schema == nil || depth == 0 {
		return nil
	}
	if name := schema.Ref.String(); name != "" {
		if pos := strings.LastIndexByte(name, '/'); pos >= 0 {
			name = name[pos+1:]
			if schema, ok := parser.swagger.Definitions[name]; ok {
				return parser.GetSchemaTypePath(&schema, depth)
			}
		}
	} else if len(schema.Type) > 0 {
		if schema.Type[0] == "array" {
			depth--
			s := []string{schema.Type[0]}
			return append(s, parser.GetSchemaTypePath(schema.Items.Schema, depth)...)
		} else if schema.Type[0] == OBJECT {
			if schema.AdditionalProperties != nil && schema.AdditionalProperties.Schema != nil {
				// for map
				depth--
				s := []string{schema.Type[0]}
				return append(s, parser.GetSchemaTypePath(schema.AdditionalProperties.Schema, depth)...)
			}
		}
		return []string{schema.Type[0]}
	}
	return nil
}

func replaceLastTag(slice []spec.Tag, element spec.Tag) {
	slice = slice[:len(slice)-1]
	slice = append(slice, element)
}

func getFloatTag(structTag reflect.StructTag, tagName string) (*float64, error) {
	strValue := structTag.Get(tagName)
	if strValue == "" {
		return nil, nil
	}

	value, err := strconv.ParseFloat(strValue, 64)
	if err != nil {
		return nil, fmt.Errorf("can't parse numeric value of %q tag: %v", tagName, err)
	}

	return &value, nil
}

func getIntTag(structTag reflect.StructTag, tagName string) (*int64, error) {
	strValue := structTag.Get(tagName)
	if strValue == "" {
		return nil, nil
	}

	value, err := strconv.ParseInt(strValue, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("can't parse numeric value of %q tag: %v", tagName, err)
	}

	return &value, nil
}

func toSnakeCase(in string) string {
	runes := []rune(in)
	length := len(runes)

	var out []rune
	for i := 0; i < length; i++ {
		if i > 0 && unicode.IsUpper(runes[i]) && ((i+1 < length && unicode.IsLower(runes[i+1])) || unicode.IsLower(runes[i-1])) {
			out = append(out, '_')
		}
		out = append(out, unicode.ToLower(runes[i]))
	}
	return string(out)
}

func toLowerCamelCase(in string) string {
	runes := []rune(in)

	var out []rune
	flag := false
	for i, curr := range runes {
		if (i == 0 && unicode.IsUpper(curr)) || (flag && unicode.IsUpper(curr)) {
			out = append(out, unicode.ToLower(curr))
			flag = true
		} else {
			out = append(out, curr)
			flag = false
		}
	}

	return string(out)
}

// defineTypeOfExample example value define the type (object and array unsupported)
func defineTypeOfExample(schemaType, arrayType, exampleValue string) (interface{}, error) {
	switch schemaType {
	case STRING:
		return exampleValue, nil
	case NUMBER:
		v, err := strconv.ParseFloat(exampleValue, 64)
		if err != nil {
			return nil, fmt.Errorf("example value %s can't convert to %s err: %s", exampleValue, schemaType, err)
		}
		return v, nil
	case INTEGER:
		v, err := strconv.Atoi(exampleValue)
		if err != nil {
			return nil, fmt.Errorf("example value %s can't convert to %s err: %s", exampleValue, schemaType, err)
		}
		return v, nil
	case BOOLEAN:
		v, err := strconv.ParseBool(exampleValue)
		if err != nil {
			return nil, fmt.Errorf("example value %s can't convert to %s err: %s", exampleValue, schemaType, err)
		}
		return v, nil
	case ARRAY:
		values := strings.Split(exampleValue, ",")
		result := make([]interface{}, 0)
		for _, value := range values {
			v, err := defineTypeOfExample(arrayType, "", value)
			if err != nil {
				return nil, err
			}
			result = append(result, v)
		}
		return result, nil
	case OBJECT:
		if arrayType == "" {
			return nil, fmt.Errorf("%s is unsupported type in example value", schemaType)
		}

		values := strings.Split(exampleValue, ",")
		result := map[string]interface{}{}
		for _, value := range values {
			mapData := strings.Split(value, ":")

			if len(mapData) == 2 {
				v, err := defineTypeOfExample(arrayType, "", mapData[1])
				if err != nil {
					return nil, err
				}
				result[mapData[0]] = v
			} else {
				return nil, fmt.Errorf("example value %s should format: key:value", exampleValue)
			}
		}
		return result, nil
	default:
		return nil, fmt.Errorf("%s is unsupported type in example value", schemaType)
	}
}

// GetAllGoFileInfo gets all Go source files information for given searchDir.
func (parser *Parser) getAllGoFileInfo(packageDir, searchDir string) error {
	return filepath.Walk(searchDir, func(path string, f os.FileInfo, err error) error {
		if err := parser.Skip(path, f); err != nil {
			return err
		} else if f.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(searchDir, path)
		if err != nil {
			return err
		}
		return parser.parseFile(filepath.ToSlash(filepath.Dir(filepath.Clean(filepath.Join(packageDir, relPath)))), path, nil)
	})
}

func (parser *Parser) getAllGoFileInfoFromDeps(pkg *depth.Pkg) error {
	ignoreInternal := pkg.Internal && !parser.ParseInternal
	if ignoreInternal || !pkg.Resolved { // ignored internal and not resolved dependencies
		return nil
	}

	// Skip cgo
	if pkg.Raw == nil && pkg.Name == "C" {
		return nil
	}
	srcDir := pkg.Raw.Dir
	files, err := ioutil.ReadDir(srcDir) // only parsing files in the dir(don't contains sub dir files)
	if err != nil {
		return err
	}

	for _, f := range files {
		if f.IsDir() {
			continue
		}

		path := filepath.Join(srcDir, f.Name())
		if err := parser.parseFile(pkg.Name, path, nil); err != nil {
			return err
		}
	}

	for i := 0; i < len(pkg.Deps); i++ {
		if err := parser.getAllGoFileInfoFromDeps(&pkg.Deps[i]); err != nil {
			return err
		}
	}

	return nil
}

func (parser *Parser) parseFile(packageDir, path string, src interface{}) error {
	if strings.HasSuffix(strings.ToLower(path), "_test.go") || filepath.Ext(path) != ".go" {
		return nil
	}

	// positions are relative to FileSet
	astFile, err := goparser.ParseFile(token.NewFileSet(), path, src, goparser.ParseComments)
	if err != nil {
		return fmt.Errorf("ParseFile error:%+v", err)
	}
	parser.packages.CollectAstFile(packageDir, path, astFile)
	return nil
}

func (parser *Parser) checkOperationIDUniqueness() error {
	// operationsIds contains all operationId annotations to check it's unique
	operationsIds := make(map[string]string)
	saveOperationID := func(operationID, currentPath string) error {
		if operationID == "" {
			return nil
		}
		if previousPath, ok := operationsIds[operationID]; ok {
			return fmt.Errorf(
				"duplicated @id annotation '%s' found in '%s', previously declared in: '%s'",
				operationID, currentPath, previousPath)
		}
		operationsIds[operationID] = currentPath
		return nil
	}
	getOperationID := func(itm spec.PathItem) (string, string) {
		if itm.Get != nil {
			return "GET", itm.Get.ID
		}
		if itm.Put != nil {
			return "PUT", itm.Put.ID
		}
		if itm.Post != nil {
			return "POST", itm.Post.ID
		}
		if itm.Delete != nil {
			return "DELETE", itm.Delete.ID
		}
		if itm.Options != nil {
			return "OPTIONS", itm.Options.ID
		}
		if itm.Head != nil {
			return "HEAD", itm.Head.ID
		}
		if itm.Patch != nil {
			return "PATCH", itm.Patch.ID
		}
		return "", ""
	}
	for path, itm := range parser.swagger.Paths.Paths {
		method, id := getOperationID(itm)
		if err := saveOperationID(id, fmt.Sprintf("%s %s", method, path)); err != nil {
			return err
		}
	}
	return nil
}

// Skip returns filepath.SkipDir error if match vendor and hidden folder
func (parser *Parser) Skip(path string, f os.FileInfo) error {
	if f.IsDir() {
		if !parser.ParseVendor && f.Name() == "vendor" || //ignore "vendor"
			f.Name() == "docs" || //exclude docs
			len(f.Name()) > 1 && f.Name()[0] == '.' { // exclude all hidden folder
			return filepath.SkipDir
		}

		if parser.excludes != nil {
			if _, ok := parser.excludes[path]; ok {
				return filepath.SkipDir
			}
		}
	}

	return nil
}

// GetSwagger returns *spec.Swagger which is the root document object for the API specification.
func (parser *Parser) GetSwagger() *spec.Swagger {
	return parser.swagger
}

//addTestType just for tests
func (parser *Parser) addTestType(typename string) {
	if parser.parsedSchemas == nil {
		parser.parsedSchemas = make(map[*TypeSpecDef]*Schema)
	}
	if parser.packages.uniqueDefinitions == nil {
		parser.packages.uniqueDefinitions = make(map[string]*TypeSpecDef)
	}
	typeDef := &TypeSpecDef{}
	parser.packages.uniqueDefinitions[typename] = typeDef
	parser.parsedSchemas[typeDef] = &Schema{
		PkgPath: "",
		Name:    typename,
		Schema:  PrimitiveSchema(OBJECT),
	}
}
