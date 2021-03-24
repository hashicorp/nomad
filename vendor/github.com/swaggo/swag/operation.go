package swag

import (
	"encoding/json"
	"fmt"
	"go/ast"
	goparser "go/parser"
	"go/token"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-openapi/spec"
	"golang.org/x/tools/go/loader"
)

// Operation describes a single API operation on a path.
// For more information: https://github.com/swaggo/swag#api-operation
type Operation struct {
	HTTPMethod string
	Path       string
	spec.Operation

	parser              *Parser
	codeExampleFilesDir string
}

var mimeTypeAliases = map[string]string{
	"json":                  "application/json",
	"xml":                   "text/xml",
	"plain":                 "text/plain",
	"html":                  "text/html",
	"mpfd":                  "multipart/form-data",
	"x-www-form-urlencoded": "application/x-www-form-urlencoded",
	"json-api":              "application/vnd.api+json",
	"json-stream":           "application/x-json-stream",
	"octet-stream":          "application/octet-stream",
	"png":                   "image/png",
	"jpeg":                  "image/jpeg",
	"gif":                   "image/gif",
}

var mimeTypePattern = regexp.MustCompile("^[^/]+/[^/]+$")

// NewOperation creates a new Operation with default properties.
// map[int]Response
func NewOperation(parser *Parser, options ...func(*Operation)) *Operation {
	if parser == nil {
		parser = New()
	}

	result := &Operation{
		parser:     parser,
		HTTPMethod: "get",
		Operation: spec.Operation{
			OperationProps: spec.OperationProps{},
			VendorExtensible: spec.VendorExtensible{
				Extensions: spec.Extensions{},
			},
		},
	}

	for _, option := range options {
		option(result)
	}

	return result
}

// SetCodeExampleFilesDirectory sets the directory to search for codeExamples
func SetCodeExampleFilesDirectory(directoryPath string) func(*Operation) {
	return func(o *Operation) {
		o.codeExampleFilesDir = directoryPath
	}
}

// ParseComment parses comment for given comment string and returns error if error occurs.
func (operation *Operation) ParseComment(comment string, astFile *ast.File) error {
	commentLine := strings.TrimSpace(strings.TrimLeft(comment, "//"))
	if len(commentLine) == 0 {
		return nil
	}
	attribute := strings.Fields(commentLine)[0]
	lineRemainder := strings.TrimSpace(commentLine[len(attribute):])
	lowerAttribute := strings.ToLower(attribute)

	var err error
	switch lowerAttribute {
	case "@description":
		operation.ParseDescriptionComment(lineRemainder)
	case "@description.markdown":
		commentInfo, err := getMarkdownForTag(lineRemainder, operation.parser.markdownFileDir)
		if err != nil {
			return err
		}
		operation.ParseDescriptionComment(string(commentInfo))
	case "@summary":
		operation.Summary = lineRemainder
	case "@id":
		operation.ID = lineRemainder
	case "@tags":
		operation.ParseTagsComment(lineRemainder)
	case "@accept":
		err = operation.ParseAcceptComment(lineRemainder)
	case "@produce":
		err = operation.ParseProduceComment(lineRemainder)
	case "@param":
		err = operation.ParseParamComment(lineRemainder, astFile)
	case "@success", "@failure", "@response":
		err = operation.ParseResponseComment(lineRemainder, astFile)
	case "@header":
		err = operation.ParseResponseHeaderComment(lineRemainder, astFile)
	case "@router":
		err = operation.ParseRouterComment(lineRemainder)
	case "@security":
		err = operation.ParseSecurityComment(lineRemainder)
	case "@deprecated":
		operation.Deprecate()
	case "@x-codesamples":
		err = operation.ParseCodeSample(attribute, commentLine, lineRemainder)
	default:
		err = operation.ParseMetadata(attribute, lowerAttribute, lineRemainder)
	}
	return err
}

// ParseCodeSample godoc
func (operation *Operation) ParseCodeSample(attribute, commentLine, lineRemainder string) error {
	if lineRemainder == "file" {
		data, err := getCodeExampleForSummary(operation.Summary, operation.codeExampleFilesDir)
		if err != nil {
			return err
		}

		var valueJSON interface{}
		if err := json.Unmarshal(data, &valueJSON); err != nil {
			return fmt.Errorf("annotation %s need a valid json value", attribute)
		}

		operation.Extensions[attribute[1:]] = valueJSON // don't use the method provided by spec lib, cause it will call toLower() on attribute names, which is wrongy

		return nil
	}

	// Fallback into existing logic
	return operation.ParseMetadata(attribute, strings.ToLower(attribute), lineRemainder)
}

// ParseDescriptionComment godoc
func (operation *Operation) ParseDescriptionComment(lineRemainder string) {
	if operation.Description == "" {
		operation.Description = lineRemainder
		return
	}
	operation.Description += "\n" + lineRemainder
}

// ParseMetadata godoc
func (operation *Operation) ParseMetadata(attribute, lowerAttribute, lineRemainder string) error {
	// parsing specific meta data extensions
	if strings.HasPrefix(lowerAttribute, "@x-") {
		if len(lineRemainder) == 0 {
			return fmt.Errorf("annotation %s need a value", attribute)
		}

		var valueJSON interface{}
		if err := json.Unmarshal([]byte(lineRemainder), &valueJSON); err != nil {
			return fmt.Errorf("annotation %s need a valid json value", attribute)
		}

		operation.Extensions[attribute[1:]] = valueJSON // don't use the method provided by spec lib, cause it will call toLower() on attribute names, which is wrongy
	}
	return nil
}

var paramPattern = regexp.MustCompile(`(\S+)[\s]+([\w]+)[\s]+([\S.]+)[\s]+([\w]+)[\s]+"([^"]+)"`)

// ParseParamComment parses params return []string of param properties
// E.g. @Param	queryText		formData	      string	  true		        "The email for login"
//              [param name]    [paramType] [data type]  [is mandatory?]   [Comment]
// E.g. @Param   some_id     path    int     true        "Some ID"
func (operation *Operation) ParseParamComment(commentLine string, astFile *ast.File) error {
	matches := paramPattern.FindStringSubmatch(commentLine)
	if len(matches) != 6 {
		return fmt.Errorf("missing required param comment parameters \"%s\"", commentLine)
	}
	name := matches[1]
	paramType := matches[2]
	refType := TransToValidSchemeType(matches[3])

	// Detect refType
	objectType := OBJECT
	if strings.HasPrefix(refType, "[]") {
		objectType = ARRAY
		refType = strings.TrimPrefix(refType, "[]")
		refType = TransToValidSchemeType(refType)
	} else if IsPrimitiveType(refType) ||
		paramType == "formData" && refType == "file" {
		objectType = PRIMITIVE
	}

	requiredText := strings.ToLower(matches[4])
	required := requiredText == "true" || requiredText == "required"
	description := matches[5]

	param := createParameter(paramType, description, name, refType, required)

	switch paramType {
	case "path", "header":
		switch objectType {
		case ARRAY, OBJECT:
			return fmt.Errorf("%s is not supported type for %s", refType, paramType)
		}
	case "query", "formData":
		switch objectType {
		case ARRAY:
			if !IsPrimitiveType(refType) {
				return fmt.Errorf("%s is not supported array type for %s", refType, paramType)
			}
			param.SimpleSchema.Type = objectType
			if operation.parser != nil {
				param.CollectionFormat = TransToValidCollectionFormat(operation.parser.collectionFormatInQuery)
			}
			param.SimpleSchema.Items = &spec.Items{
				SimpleSchema: spec.SimpleSchema{
					Type: refType,
				},
			}
		case OBJECT:
			schema, err := operation.parser.getTypeSchema(refType, astFile, false)
			if err != nil {
				return err
			}
			if len(schema.Properties) == 0 {
				return nil
			}
			find := func(arr []string, target string) bool {
				for _, str := range arr {
					if str == target {
						return true
					}
				}
				return false
			}
			items := schema.Properties.ToOrderedSchemaItems()
			for _, item := range items {
				name := item.Name
				prop := item.Schema
				if len(prop.Type) == 0 {
					continue
				}
				if prop.Type[0] == ARRAY &&
					prop.Items.Schema != nil &&
					len(prop.Items.Schema.Type) > 0 &&
					IsSimplePrimitiveType(prop.Items.Schema.Type[0]) {
					param = createParameter(paramType, prop.Description, name, prop.Type[0], find(schema.Required, name))
					param.SimpleSchema.Type = prop.Type[0]
					if operation.parser != nil && operation.parser.collectionFormatInQuery != "" && param.CollectionFormat == "" {
						param.CollectionFormat = TransToValidCollectionFormat(operation.parser.collectionFormatInQuery)
					}
					param.SimpleSchema.Items = &spec.Items{
						SimpleSchema: spec.SimpleSchema{
							Type: prop.Items.Schema.Type[0],
						},
					}
				} else if IsSimplePrimitiveType(prop.Type[0]) {
					param = createParameter(paramType, prop.Description, name, prop.Type[0], find(schema.Required, name))
				} else {
					Println(fmt.Sprintf("skip field [%s] in %s is not supported type for %s", name, refType, paramType))
					continue
				}
				param.Nullable = prop.Nullable
				param.Format = prop.Format
				param.Default = prop.Default
				param.Example = prop.Example
				param.Extensions = prop.Extensions
				param.CommonValidations.Maximum = prop.Maximum
				param.CommonValidations.Minimum = prop.Minimum
				param.CommonValidations.ExclusiveMaximum = prop.ExclusiveMaximum
				param.CommonValidations.ExclusiveMinimum = prop.ExclusiveMinimum
				param.CommonValidations.MaxLength = prop.MaxLength
				param.CommonValidations.MinLength = prop.MinLength
				param.CommonValidations.Pattern = prop.Pattern
				param.CommonValidations.MaxItems = prop.MaxItems
				param.CommonValidations.MinItems = prop.MinItems
				param.CommonValidations.UniqueItems = prop.UniqueItems
				param.CommonValidations.MultipleOf = prop.MultipleOf
				param.CommonValidations.Enum = prop.Enum
				operation.Operation.Parameters = append(operation.Operation.Parameters, param)
			}
			return nil
		}
	case "body":
		schema, err := operation.parseAPIObjectSchema(objectType, refType, astFile)
		if err != nil {
			return err
		}
		param.Schema = schema
	default:
		return fmt.Errorf("%s is not supported paramType", paramType)
	}

	if err := operation.parseAndExtractionParamAttribute(commentLine, objectType, refType, &param); err != nil {
		return err
	}
	operation.Operation.Parameters = append(operation.Operation.Parameters, param)
	return nil
}

var regexAttributes = map[string]*regexp.Regexp{
	// for Enums(A, B)
	"enums": regexp.MustCompile(`(?i)\s+enums\(.*\)`),
	// for maximum(0)
	"maximum": regexp.MustCompile(`(?i)\s+maxinum|maximum\(.*\)`),
	// for minimum(0)
	"minimum": regexp.MustCompile(`(?i)\s+mininum|minimum\(.*\)`),
	// for default(0)
	"default": regexp.MustCompile(`(?i)\s+default\(.*\)`),
	// for minlength(0)
	"minlength": regexp.MustCompile(`(?i)\s+minlength\(.*\)`),
	// for maxlength(0)
	"maxlength": regexp.MustCompile(`(?i)\s+maxlength\(.*\)`),
	// for format(email)
	"format": regexp.MustCompile(`(?i)\s+format\(.*\)`),
	// for collectionFormat(csv)
	"collectionFormat": regexp.MustCompile(`(?i)\s+collectionFormat\(.*\)`),
}

func (operation *Operation) parseAndExtractionParamAttribute(commentLine, objectType, schemaType string, param *spec.Parameter) error {
	schemaType = TransToValidSchemeType(schemaType)
	for attrKey, re := range regexAttributes {
		attr, err := findAttr(re, commentLine)
		if err != nil {
			continue
		}
		switch attrKey {
		case "enums":
			err := setEnumParam(attr, schemaType, param)
			if err != nil {
				return err
			}
		case "maximum":
			n, err := setNumberParam(attrKey, schemaType, attr, commentLine)
			if err != nil {
				return err
			}
			param.Maximum = &n
		case "minimum":
			n, err := setNumberParam(attrKey, schemaType, attr, commentLine)
			if err != nil {
				return err
			}
			param.Minimum = &n
		case "default":
			value, err := defineType(schemaType, attr)
			if err != nil {
				return nil
			}
			param.Default = value
		case "maxlength":
			n, err := setStringParam(attrKey, schemaType, attr, commentLine)
			if err != nil {
				return err
			}
			param.MaxLength = &n
		case "minlength":
			n, err := setStringParam(attrKey, schemaType, attr, commentLine)
			if err != nil {
				return err
			}
			param.MinLength = &n
		case "format":
			param.Format = attr
		case "collectionFormat":
			n, err := setCollectionFormatParam(attrKey, objectType, attr, commentLine)
			if err != nil {
				return err
			}
			param.CollectionFormat = n
		}
	}
	return nil
}

func findAttr(re *regexp.Regexp, commentLine string) (string, error) {
	attr := re.FindString(commentLine)
	l := strings.Index(attr, "(")
	r := strings.Index(attr, ")")
	if l == -1 || r == -1 {
		return "", fmt.Errorf("can not find regex=%s, comment=%s", re.String(), commentLine)
	}
	return strings.TrimSpace(attr[l+1 : r]), nil
}

func setStringParam(name, schemaType, attr, commentLine string) (int64, error) {
	if schemaType != STRING {
		return 0, fmt.Errorf("%s is attribute to set to a number. comment=%s got=%s", name, commentLine, schemaType)
	}
	n, err := strconv.ParseInt(attr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s is allow only a number got=%s", name, attr)
	}
	return n, nil
}

func setNumberParam(name, schemaType, attr, commentLine string) (float64, error) {
	if schemaType != INTEGER && schemaType != NUMBER {
		return 0, fmt.Errorf("%s is attribute to set to a number. comment=%s got=%s", name, commentLine, schemaType)
	}
	n, err := strconv.ParseFloat(attr, 64)
	if err != nil {
		return 0, fmt.Errorf("maximum is allow only a number. comment=%s got=%s", commentLine, attr)
	}
	return n, nil
}

func setEnumParam(attr, schemaType string, param *spec.Parameter) error {
	for _, e := range strings.Split(attr, ",") {
		e = strings.TrimSpace(e)

		value, err := defineType(schemaType, e)
		if err != nil {
			return err
		}
		param.Enum = append(param.Enum, value)
	}
	return nil
}

func setCollectionFormatParam(name, schemaType, attr, commentLine string) (string, error) {
	if schemaType != ARRAY {
		return "", fmt.Errorf("%s is attribute to set to an array. comment=%s got=%s", name, commentLine, schemaType)
	}
	return TransToValidCollectionFormat(attr), nil
}

// defineType enum value define the type (object and array unsupported)
func defineType(schemaType string, value string) (interface{}, error) {
	schemaType = TransToValidSchemeType(schemaType)
	switch schemaType {
	case STRING:
		return value, nil
	case NUMBER:
		v, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, fmt.Errorf("enum value %s can't convert to %s err: %s", value, schemaType, err)
		}
		return v, nil
	case INTEGER:
		v, err := strconv.Atoi(value)
		if err != nil {
			return nil, fmt.Errorf("enum value %s can't convert to %s err: %s", value, schemaType, err)
		}
		return v, nil
	case BOOLEAN:
		v, err := strconv.ParseBool(value)
		if err != nil {
			return nil, fmt.Errorf("enum value %s can't convert to %s err: %s", value, schemaType, err)
		}
		return v, nil
	default:
		return nil, fmt.Errorf("%s is unsupported type in enum value", schemaType)
	}
}

// ParseTagsComment parses comment for given `tag` comment string.
func (operation *Operation) ParseTagsComment(commentLine string) {
	tags := strings.Split(commentLine, ",")
	for _, tag := range tags {
		operation.Tags = append(operation.Tags, strings.TrimSpace(tag))
	}
}

// ParseAcceptComment parses comment for given `accept` comment string.
func (operation *Operation) ParseAcceptComment(commentLine string) error {
	return parseMimeTypeList(commentLine, &operation.Consumes, "%v accept type can't be accepted")
}

// ParseProduceComment parses comment for given `produce` comment string.
func (operation *Operation) ParseProduceComment(commentLine string) error {
	return parseMimeTypeList(commentLine, &operation.Produces, "%v produce type can't be accepted")
}

// parseMimeTypeList parses a list of MIME Types for a comment like
// `produce` (`Content-Type:` response header) or
// `accept` (`Accept:` request header)
func parseMimeTypeList(mimeTypeList string, typeList *[]string, format string) error {
	mimeTypes := strings.Split(mimeTypeList, ",")
	for _, typeName := range mimeTypes {
		if mimeTypePattern.MatchString(typeName) {
			*typeList = append(*typeList, typeName)
			continue
		}
		if aliasMimeType, ok := mimeTypeAliases[typeName]; ok {
			*typeList = append(*typeList, aliasMimeType)
			continue
		}
		return fmt.Errorf(format, typeName)
	}
	return nil
}

var routerPattern = regexp.MustCompile(`^(/[\w\.\/\-{}\+:]*)[[:blank:]]+\[(\w+)]`)

// ParseRouterComment parses comment for gived `router` comment string.
func (operation *Operation) ParseRouterComment(commentLine string) error {
	var matches []string

	if matches = routerPattern.FindStringSubmatch(commentLine); len(matches) != 3 {
		return fmt.Errorf("can not parse router comment \"%s\"", commentLine)
	}
	path := matches[1]
	httpMethod := matches[2]

	operation.Path = path
	operation.HTTPMethod = strings.ToUpper(httpMethod)

	return nil
}

// ParseSecurityComment parses comment for gived `security` comment string.
func (operation *Operation) ParseSecurityComment(commentLine string) error {
	securitySource := commentLine[strings.Index(commentLine, "@Security")+1:]
	l := strings.Index(securitySource, "[")
	r := strings.Index(securitySource, "]")
	// exists scope
	if !(l == -1 && r == -1) {
		scopes := securitySource[l+1 : r]
		s := []string{}
		for _, scope := range strings.Split(scopes, ",") {
			scope = strings.TrimSpace(scope)
			s = append(s, scope)
		}
		securityKey := securitySource[0:l]
		securityMap := map[string][]string{}
		securityMap[securityKey] = append(securityMap[securityKey], s...)
		operation.Security = append(operation.Security, securityMap)
	} else {
		securityKey := strings.TrimSpace(securitySource)
		securityMap := map[string][]string{}
		securityMap[securityKey] = []string{}
		operation.Security = append(operation.Security, securityMap)
	}
	return nil
}

// findTypeDef attempts to find the *ast.TypeSpec for a specific type given the
// type's name and the package's import path
// TODO: improve finding external pkg
func findTypeDef(importPath, typeName string) (*ast.TypeSpec, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	conf := loader.Config{
		ParserMode: goparser.SpuriousErrors,
		Cwd:        cwd,
	}

	conf.Import(importPath)

	lprog, err := conf.Load()
	if err != nil {
		return nil, err
	}

	// If the pkg is vendored, the actual pkg path is going to resemble
	// something like "{importPath}/vendor/{importPath}"
	for k := range lprog.AllPackages {
		realPkgPath := k.Path()

		if strings.Contains(realPkgPath, "vendor/"+importPath) {
			importPath = realPkgPath
		}
	}

	pkgInfo := lprog.Package(importPath)

	if pkgInfo == nil {
		return nil, fmt.Errorf("package was nil")
	}

	// TODO: possibly cache pkgInfo since it's an expensive operation

	for i := range pkgInfo.Files {
		for _, astDeclaration := range pkgInfo.Files[i].Decls {
			if generalDeclaration, ok := astDeclaration.(*ast.GenDecl); ok && generalDeclaration.Tok == token.TYPE {
				for _, astSpec := range generalDeclaration.Specs {
					if typeSpec, ok := astSpec.(*ast.TypeSpec); ok {
						if typeSpec.Name.String() == typeName {
							return typeSpec, nil
						}
					}
				}
			}
		}
	}
	return nil, fmt.Errorf("type spec not found")
}

var responsePattern = regexp.MustCompile(`([\w,]+)[\s]+([\w\{\}]+)[\s]+([\w\-\.\/\{\}=,\[\]]+)[^"]*(.*)?`)

//RepsonseType{data1=Type1,data2=Type2}
var combinedPattern = regexp.MustCompile(`^([\w\-\.\/\[\]]+)\{(.*)\}$`)

func (operation *Operation) parseObjectSchema(refType string, astFile *ast.File) (*spec.Schema, error) {
	switch {
	case refType == "interface{}":
		return PrimitiveSchema(OBJECT), nil
	case IsGolangPrimitiveType(refType):
		refType = TransToValidSchemeType(refType)
		return PrimitiveSchema(refType), nil
	case IsPrimitiveType(refType):
		return PrimitiveSchema(refType), nil
	case strings.HasPrefix(refType, "[]"):
		schema, err := operation.parseObjectSchema(refType[2:], astFile)
		if err != nil {
			return nil, err
		}
		return spec.ArrayProperty(schema), nil
	case strings.HasPrefix(refType, "map["):
		//ignore key type
		idx := strings.Index(refType, "]")
		if idx < 0 {
			return nil, fmt.Errorf("invalid type: %s", refType)
		}
		refType = refType[idx+1:]
		if refType == "interface{}" {
			return spec.MapProperty(nil), nil

		}
		schema, err := operation.parseObjectSchema(refType, astFile)
		if err != nil {
			return nil, err
		}
		return spec.MapProperty(schema), nil
	case strings.Contains(refType, "{"):
		return operation.parseCombinedObjectSchema(refType, astFile)
	default:
		if operation.parser != nil { // checking refType has existing in 'TypeDefinitions'
			schema, err := operation.parser.getTypeSchema(refType, astFile, true)
			if err != nil {
				return nil, err
			}
			return schema, nil
		}

		return RefSchema(refType), nil
	}
}

func (operation *Operation) parseCombinedObjectSchema(refType string, astFile *ast.File) (*spec.Schema, error) {
	matches := combinedPattern.FindStringSubmatch(refType)
	if len(matches) != 3 {
		return nil, fmt.Errorf("invalid type: %s", refType)
	}
	refType = matches[1]
	schema, err := operation.parseObjectSchema(refType, astFile)
	if err != nil {
		return nil, err
	}

	parseFields := func(s string) []string {
		n := 0
		return strings.FieldsFunc(s, func(r rune) bool {
			if r == '{' {
				n++
				return false
			} else if r == '}' {
				n--
				return false
			}
			return r == ',' && n == 0
		})
	}

	fields := parseFields(matches[2])
	props := map[string]spec.Schema{}
	for _, field := range fields {
		if matches := strings.SplitN(field, "=", 2); len(matches) == 2 {
			schema, err := operation.parseObjectSchema(matches[1], astFile)
			if err != nil {
				return nil, err
			}
			props[matches[0]] = *schema
		}
	}

	if len(props) == 0 {
		return schema, nil
	}
	return spec.ComposedSchema(*schema, spec.Schema{
		SchemaProps: spec.SchemaProps{
			Type:       []string{OBJECT},
			Properties: props,
		},
	}), nil
}

func (operation *Operation) parseAPIObjectSchema(schemaType, refType string, astFile *ast.File) (*spec.Schema, error) {
	switch schemaType {
	case OBJECT:
		if !strings.HasPrefix(refType, "[]") {
			return operation.parseObjectSchema(refType, astFile)
		}
		refType = refType[2:]
		fallthrough
	case ARRAY:
		schema, err := operation.parseObjectSchema(refType, astFile)
		if err != nil {
			return nil, err
		}
		return spec.ArrayProperty(schema), nil
	case PRIMITIVE:
		return PrimitiveSchema(refType), nil
	default:
		return PrimitiveSchema(schemaType), nil
	}
}

// ParseResponseComment parses comment for given `response` comment string.
func (operation *Operation) ParseResponseComment(commentLine string, astFile *ast.File) error {
	var matches []string

	if matches = responsePattern.FindStringSubmatch(commentLine); len(matches) != 5 {
		err := operation.ParseEmptyResponseComment(commentLine)
		if err != nil {
			return operation.ParseEmptyResponseOnly(commentLine)
		}
		return err
	}

	responseDescription := strings.Trim(matches[4], "\"")
	schemaType := strings.Trim(matches[2], "{}")
	refType := matches[3]
	schema, err := operation.parseAPIObjectSchema(schemaType, refType, astFile)
	if err != nil {
		return err
	}

	for _, codeStr := range strings.Split(matches[1], ",") {
		if strings.EqualFold(codeStr, "default") {
			operation.DefaultResponse().Schema = schema
			operation.DefaultResponse().Description = responseDescription
		} else if code, err := strconv.Atoi(codeStr); err == nil {
			if responseDescription == "" {
				responseDescription = http.StatusText(code)
			}

			operation.AddResponse(code, &spec.Response{
				ResponseProps: spec.ResponseProps{Schema: schema, Description: responseDescription},
			})
		} else {
			return fmt.Errorf("can not parse response comment \"%s\"", commentLine)
		}
	}

	return nil
}

// ParseResponseHeaderComment parses comment for gived `response header` comment string.
func (operation *Operation) ParseResponseHeaderComment(commentLine string, astFile *ast.File) error {
	var matches []string

	if matches = responsePattern.FindStringSubmatch(commentLine); len(matches) != 5 {
		return fmt.Errorf("can not parse response comment \"%s\"", commentLine)
	}

	schemaType := strings.Trim(matches[2], "{}")
	headerKey := matches[3]
	description := strings.Trim(matches[4], "\"")
	header := spec.Header{}
	header.Description = description
	header.Type = schemaType

	if strings.EqualFold(matches[1], "all") {
		if operation.Responses.Default != nil {
			if operation.Responses.Default.Headers == nil {
				operation.Responses.Default.Headers = make(map[string]spec.Header)
			}
			operation.Responses.Default.Headers[headerKey] = header
		}
		if operation.Responses != nil && operation.Responses.StatusCodeResponses != nil {
			for code, response := range operation.Responses.StatusCodeResponses {
				if response.Headers == nil {
					response.Headers = make(map[string]spec.Header)
				}
				response.Headers[headerKey] = header
				operation.Responses.StatusCodeResponses[code] = response
			}
		}
		return nil
	}

	for _, codeStr := range strings.Split(matches[1], ",") {
		if strings.EqualFold(codeStr, "default") {
			if operation.Responses.Default != nil {
				if operation.Responses.Default.Headers == nil {
					operation.Responses.Default.Headers = make(map[string]spec.Header)
				}
				operation.Responses.Default.Headers[headerKey] = header
			}
		} else if code, err := strconv.Atoi(codeStr); err == nil {
			if operation.Responses != nil && operation.Responses.StatusCodeResponses != nil {
				if response, responseExist := operation.Responses.StatusCodeResponses[code]; responseExist {
					if response.Headers == nil {
						response.Headers = make(map[string]spec.Header)
					}
					response.Headers[headerKey] = header

					operation.Responses.StatusCodeResponses[code] = response
				}
			}
		} else {
			return fmt.Errorf("can not parse response comment \"%s\"", commentLine)
		}
	}

	return nil
}

var emptyResponsePattern = regexp.MustCompile(`([\w,]+)[\s]+"(.*)"`)

// ParseEmptyResponseComment parse only comment out status code and description,eg: @Success 200 "it's ok"
func (operation *Operation) ParseEmptyResponseComment(commentLine string) error {
	var matches []string

	if matches = emptyResponsePattern.FindStringSubmatch(commentLine); len(matches) != 3 {
		return fmt.Errorf("can not parse response comment \"%s\"", commentLine)
	}

	responseDescription := strings.Trim(matches[2], "\"")
	for _, codeStr := range strings.Split(matches[1], ",") {
		if strings.EqualFold(codeStr, "default") {
			operation.DefaultResponse().Description = responseDescription
		} else if code, err := strconv.Atoi(codeStr); err == nil {
			var response spec.Response
			response.Description = responseDescription
			operation.AddResponse(code, &response)
		} else {
			return fmt.Errorf("can not parse response comment \"%s\"", commentLine)
		}
	}

	return nil
}

//ParseEmptyResponseOnly parse only comment out status code ,eg: @Success 200
func (operation *Operation) ParseEmptyResponseOnly(commentLine string) error {
	for _, codeStr := range strings.Split(commentLine, ",") {
		if strings.EqualFold(codeStr, "default") {
			_ = operation.DefaultResponse()
		} else if code, err := strconv.Atoi(codeStr); err == nil {
			var response spec.Response
			//response.Description = http.StatusText(code)
			operation.AddResponse(code, &response)
		} else {
			return fmt.Errorf("can not parse response comment \"%s\"", commentLine)
		}
	}

	return nil
}

//DefaultResponse return the default response member pointer
func (operation *Operation) DefaultResponse() *spec.Response {
	if operation.Responses.Default == nil {
		operation.Responses.Default = &spec.Response{}
	}
	return operation.Responses.Default
}

//AddResponse add a response for a code
func (operation *Operation) AddResponse(code int, response *spec.Response) {
	if operation.Responses == nil {
		operation.Responses = &spec.Responses{
			ResponsesProps: spec.ResponsesProps{
				StatusCodeResponses: make(map[int]spec.Response),
			},
		}
	}
	operation.Responses.StatusCodeResponses[code] = *response
}

// createParameter returns swagger spec.Parameter for gived  paramType, description, paramName, schemaType, required
func createParameter(paramType, description, paramName, schemaType string, required bool) spec.Parameter {
	// //five possible parameter types. 	query, path, body, header, form
	paramProps := spec.ParamProps{
		Name:        paramName,
		Description: description,
		Required:    required,
		In:          paramType,
	}
	if paramType == "body" {
		paramProps.Schema = &spec.Schema{
			SchemaProps: spec.SchemaProps{
				Type: []string{schemaType},
			},
		}
		parameter := spec.Parameter{
			ParamProps: paramProps,
		}
		return parameter
	}
	parameter := spec.Parameter{
		ParamProps: paramProps,
		SimpleSchema: spec.SimpleSchema{
			Type: schemaType,
		},
	}
	return parameter
}

func getCodeExampleForSummary(summaryName string, dirPath string) ([]byte, error) {
	filesInfos, err := ioutil.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	for _, fileInfo := range filesInfos {
		if fileInfo.IsDir() {
			continue
		}
		fileName := fileInfo.Name()

		if !strings.Contains(fileName, ".json") {
			continue
		}

		if strings.Contains(fileName, summaryName) {
			fullPath := filepath.Join(dirPath, fileName)
			commentInfo, err := ioutil.ReadFile(fullPath)
			if err != nil {
				return nil, fmt.Errorf("Failed to read code example file %s error: %s ", fullPath, err)
			}
			return commentInfo, nil
		}
	}
	return nil, fmt.Errorf("Unable to find code example file for tag %s in the given directory", summaryName)
}
