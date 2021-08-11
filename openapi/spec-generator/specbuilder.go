package main

import (
	"context"
	"fmt"
	"github.com/hashicorp/go-hclog"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3gen"
	"github.com/ghodss/yaml"
)

const schemaPath = "#/components/schemas"

// spec wraps a kin-openapi document object model with extra
// metadata so that the template can be entirely data driven
type spec struct {
	Model openapi3.T // Document object model we are building
}

func (s *spec) ToBytes() ([]byte, error) {
	data, err := yaml.Marshal(s.Model)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (s *spec) ToYAML() (string, error) {
	data, err := s.ToBytes()
	if err != nil {
		return "", err
	}

	return string(data), nil
}

type specBuilder struct {
	spec   *spec
	logger hclog.Logger
	kingen *openapi3gen.Generator
}

func (b *specBuilder) buildSpec() (*spec, error) {
	b.logger = hclog.Default()
	b.kingen = openapi3gen.NewGenerator(openapi3gen.UseAllExportedFields())
	// TODO: Eventually may need to support multiple OpenAPI versions, but pushing
	// that off for now.
	b.spec = &spec{
		Model: openapi3.T{
			OpenAPI: "3.0.3",
		},
	}

	b.spec.Model.Info = &infoModel
	b.spec.Model.Security = securityModel
	b.spec.Model.Servers = serversModel
	b.spec.Model.Tags = tagsModel
	b.spec.Model.Components = openapi3.NewComponents()
	b.spec.Model.Paths = openapi3.Paths{}

	if err := b.buildComponentsAndPaths(); err != nil {
		return nil, err
	}

	if err := b.spec.Model.Validate(context.Background()); err != nil {
		return nil, err
	}

	return b.spec, nil
}

var infoModel = openapi3.Info{
	Version: "1.1.3", // TODO: Schlep this dynamically from VersionInfo
	Title:   "Nomad",
	Contact: &openapi3.Contact{
		Email: "support@hashicorp.com",
	},
	License: &openapi3.License{
		Name: "MPL 2",
		URL:  "https://github.com/hashicorp/nomad/blob/main/LICENSE",
	},
}

var securityModel = *openapi3.NewSecurityRequirements().With(openapi3.SecurityRequirement{
	"X-Nomad-Token": {},
})

var serversModel = openapi3.Servers{
	{
		Description: "dev-agent",
		URL:         "{scheme}://{address}:{port}/v1",
		Variables: map[string]*openapi3.ServerVariable{
			"address": {Default: "127.0.0.1"},
			"port":    {Default: "4646"},
			"scheme":  {Default: "https", Enum: []string{"https", "http"}},
		},
	},
	{
		Description: "agent",
		URL:         "{scheme}://{address}:{port}/v1",
		Variables: map[string]*openapi3.ServerVariable{
			"address": {Default: "127.0.0.1"},
			"port":    {Default: "4646"},
			"scheme":  {Default: "https", Enum: []string{"https", "http"}},
		},
	},
}

var tagsModel = openapi3.Tags{}

// buildComponentsAndPaths builds the Components object graph by parsing the model.
func (b *specBuilder) buildComponentsAndPaths() error {
	components := &b.spec.Model.Components
	components.RequestBodies = openapi3.RequestBodies{}
	components.Parameters = openapi3.ParametersMap{}
	components.Responses = openapi3.NewResponses()
	components.Schemas = openapi3.Schemas{}
	components.Headers = openapi3.Headers{}
	components.SecuritySchemes = openapi3.SecuritySchemes{}

	err := b.buildGraphFromModel()
	if err != nil {
		return err
	}

	b.resolveRefPaths()

	return nil
}

func (b *specBuilder) buildGraphFromModel() error {
	api := v1api{}
	paths := openapi3.Paths{}

	for _, path := range api.GetPaths() {
		if len(path.Template) == 0 {
			err := fmt.Errorf("invalid apiPath spec: %#v", path)
			b.logger.Error("", err)
			return err
		}

		pathItem := &openapi3.PathItem{}

		for _, pathOp := range path.Operations {
			requestBodyRef, err := b.getOrCreateRequestBody(pathOp.RequestBody)
			if err != nil {
				b.logger.Error("unable to adapt request body", err, pathOp)
				return err
			}

			responses, err := b.getOrCreateResponses(pathOp.Responses)
			if err != nil {
				b.logger.Error("unable to adapt operation responses", err, path, pathOp)
				return err
			}

			op := &openapi3.Operation{
				Tags:        pathOp.Tags,
				Summary:     pathOp.Summary,
				Description: pathOp.Description,
				OperationID: pathOp.Description,
				Parameters:  b.getOrCreateParameters(pathOp.Parameters),
				RequestBody: requestBodyRef,
				Responses:   responses,
				Security:    b.adaptSecurityRequirements(pathOp.Parameters),
			}

			switch pathOp.Method {
			case http.MethodGet:
				pathItem.Get = op
			case http.MethodPost:
				pathItem.Post = op
			case http.MethodDelete:
				pathItem.Delete = op
			}
		}

		paths[path.Template] = pathItem
	}

	b.spec.Model.Paths = paths

	return nil
}

func (b *specBuilder) getOrCreateRequestBody(requestBodyModel *requestBody) (*openapi3.RequestBodyRef, error) {
	if requestBodyModel == nil {
		return nil, nil
	}

	if existing, ok := b.spec.Model.Components.RequestBodies[requestBodyModel.Model.Name()]; ok {
		return existing, nil
	}

	body := openapi3.NewRequestBody()
	schemaRef, err := b.getOrCreateSchemaRef(requestBodyModel.Model)
	if err != nil {
		return nil, err
	}

	body.Required = true
	body.Content = openapi3.NewContentWithSchemaRef(&openapi3.SchemaRef{
		Ref:   formatSchemaRefPath(schemaRef, requestBodyModel.Model.Name()),
		Value: schemaRef.Value,
	}, []string{"application/json"})

	b.spec.Model.Components.RequestBodies[requestBodyModel.Model.Name()] = &openapi3.RequestBodyRef{
		Ref:   "",
		Value: body,
	}

	return b.spec.Model.Components.RequestBodies[requestBodyModel.Model.Name()], nil
}

func (b *specBuilder) getOrCreateResponses(configs []*responseConfig) (openapi3.Responses, error) {
	responses := openapi3.Responses{}

	for _, cfg := range configs {
		// if it isn't the global map, add it
		responseRef, ok := b.spec.Model.Components.Responses[cfg.Response.Name]
		if !ok {
			responseRef = &openapi3.ResponseRef{
				Ref: "",
				Value: &openapi3.Response{
					Description: &cfg.Response.Description,
					Headers:     b.getOrCreateHeaders(cfg.Headers),
				},
			}

			if cfg.Content != nil {
				schemaRef, err := b.getOrCreateSchemaRef(cfg.Content.Model)
				if err != nil {
					b.logger.Error("unable to getOrCreateSchemaRef for response", cfg, err)
					return nil, err
				}

				switch cfg.Content.SchemaType {
				case objectSchema:
					responseRef.Value.Content = openapi3.NewContentWithSchemaRef(&openapi3.SchemaRef{
						Ref:   formatSchemaRefPath(schemaRef, cfg.Content.Model.Name()),
						Value: schemaRef.Value,
					}, []string{"application/json"})
				case arraySchema:
					responseRef.Value.Content = openapi3.NewContentWithSchemaRef(openapi3.NewSchemaRef("", &openapi3.Schema{
						Type: string(arraySchema),
						Items: &openapi3.SchemaRef{
							Ref:   formatSchemaRefPath(schemaRef, cfg.Content.Model.Name()),
							Value: schemaRef.Value,
						},
					}), []string{"application/json"})
				}
			}
			// Now add to the global response map
			if _, ok = b.spec.Model.Components.Responses[cfg.Response.Name]; !ok {
				b.spec.Model.Components.Responses[cfg.Response.Name] = responseRef
			}
		}

		// Now add a ref for the path
		responses[strconv.Itoa(cfg.Code)] = &openapi3.ResponseRef{
			Ref:   fmt.Sprintf("#/components/responses/%s", cfg.Response.Name),
			Value: responseRef.Value,
		}
	}

	return responses, nil
}

func (b *specBuilder) getOrCreateHeaders(headers []*responseHeader) openapi3.Headers {
	headerRefs := openapi3.Headers{}

	for _, header := range headers {
		headerRef, ok := b.spec.Model.Components.Headers[header.Name]
		if !ok {
			headerRef = &openapi3.HeaderRef{
				Ref: "", //fmt.Sprintf("#/components/parameters/%s", header.Name),
				Value: &openapi3.Header{
					Parameter: openapi3.Parameter{
						Name:        header.Name,
						Description: header.Description,
						Schema: openapi3.NewSchemaRef("", &openapi3.Schema{
							Type: string(header.SchemaType),
						}),
					},
				},
			}
		}

		headerRefs[header.Name] = headerRef
	}

	return headerRefs
}

func (b *specBuilder) getOrCreateParameters(params []*parameter) openapi3.Parameters {
	parameters := openapi3.Parameters{}

	for _, param := range params {
		if existing, ok := b.spec.Model.Components.Parameters[param.Id]; ok {
			parameters = append(parameters, &openapi3.ParameterRef{
				Ref:   formatParameterRefPath(param),
				Value: existing.Value,
			})
		} else {
			parameterRef := &openapi3.ParameterRef{
				Ref: "",
				Value: &openapi3.Parameter{
					Name:        param.Name,
					Description: param.Description,
					Schema: openapi3.NewSchemaRef("", &openapi3.Schema{
						Type: string(param.SchemaType),
					}),
					In:       string(param.In),
					Required: param.Required,
				},
			}

			b.spec.Model.Components.Parameters[param.Id] = parameterRef

			parameters = append(parameters, &openapi3.ParameterRef{
				Ref:   formatParameterRefPath(param),
				Value: parameterRef.Value,
			})
		}
	}

	return parameters
}

func (b *specBuilder) adaptSecurityRequirements(parameters []*parameter) *openapi3.SecurityRequirements {
	var tokenParam *parameter
	for _, param := range parameters {
		if param.Name == "X-Nomad-Token" {
			tokenParam = param
			break
		}
	}

	if tokenParam == nil {
		return nil
	}

	var ok bool
	var securitySchemeRef *openapi3.SecuritySchemeRef
	if securitySchemeRef, ok = b.spec.Model.Components.SecuritySchemes[tokenParam.Name]; !ok {
		securitySchemeRef = &openapi3.SecuritySchemeRef{
			Ref: "",
			Value: &openapi3.SecurityScheme{
				Type: "apiKey",
				Name: tokenParam.Name,
				In:   string(tokenParam.In),
			},
		}

		b.spec.Model.Components.SecuritySchemes[tokenParam.Name] = securitySchemeRef
	}

	return &openapi3.SecurityRequirements{
		{
			tokenParam.Name: {},
		},
	}
}

// getOrCreateSchemaRef creates a SchemaRef for a Request or response content. It
// adds all referenced schemas to the schemas map, but DOES NOT add the request
// or response schema. The callers are responsible for adding the root to the
// correct map.
func (b *specBuilder) getOrCreateSchemaRef(model reflect.Type) (*openapi3.SchemaRef, error) {
	var err error
	var ref *openapi3.SchemaRef

	// if it doesn't exist generate
	ref, ok := b.spec.Model.Components.Schemas[model.Name()]
	if !ok {
		ref, err = b.kingen.GenerateSchemaRef(model)
		if err != nil {
			return nil, err
		}
	}

	// Add all generated schema refs if not already added
	for schemaRef, _ := range b.kingen.SchemaRefs {
		if isBasic(schemaRef.Ref) {
			continue
		}
		if len(schemaRef.Ref) > 0 && !strings.Contains(schemaRef.Ref, schemaPath) {
			if _, ok = b.spec.Model.Components.Schemas[schemaRef.Ref]; !ok {
				b.spec.Model.Components.Schemas[schemaRef.Ref] = openapi3.NewSchemaRef("", schemaRef.Value)
			}
		}
	}

	return ref, nil
}

func (b *specBuilder) resolveRefPaths() {
	for _, schemaRef := range b.spec.Model.Components.Schemas {
		// Next make sure the refs point to other schemas, if not already done.
		for _, propertyRef := range schemaRef.Value.Properties {
			if strings.Contains(propertyRef.Ref, schemaPath) {
				continue
			}

			if isBasic(propertyRef.Value.Type) {
				propertyRef.Ref = ""
			} else if propertyRef.Value.Type == "array" {
				propertyRef.Value.Items.Ref = formatSchemaRefPath(propertyRef.Value.Items, propertyRef.Value.Items.Ref)
			} else if propertyRef.Value.AdditionalProperties != nil {
				// This handles maps
				if len(propertyRef.Value.AdditionalProperties.Ref) != 0 {
					propertyRef.Value.AdditionalProperties.Ref = formatSchemaRefPath(propertyRef.Value.AdditionalProperties, propertyRef.Value.AdditionalProperties.Ref)
				} else if propertyRef.Value.AdditionalProperties.Value.Type == "array" {
					propertyRef.Value.AdditionalProperties.Value.Items.Ref = formatSchemaRefPath(propertyRef.Value.AdditionalProperties, propertyRef.Value.AdditionalProperties.Value.Items.Ref)
				} else if len(propertyRef.Ref) != 0 {
					propertyRef.Ref = formatSchemaRefPath(propertyRef, propertyRef.Ref)
				}
			} else {
				propertyRef.Ref = formatSchemaRefPath(propertyRef, propertyRef.Ref)
			}
		}
	}
}

func isBasic(typ string) bool {
	return typ == "" || typ == "integer" || typ == "number" || typ == "string" || typ == "boolean" || typ == "bool"
}

func formatSchemaRefPath(ref *openapi3.SchemaRef, modelName string) string {
	// If basic, return empty
	if isBasic(ref.Value.Type) {
		return ""
	}

	// If already formatted return that
	if strings.Contains(ref.Ref, schemaPath) {
		return ref.Ref
	}

	return fmt.Sprintf("#/components/schemas/%s", modelName)
}

func formatParameterRefPath(param *parameter) string {
	return fmt.Sprintf("#/components/parameters/%s", param.Id)
}
