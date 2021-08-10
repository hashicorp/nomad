package spec

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3gen"
	"github.com/ghodss/yaml"
)

// Spec wraps a kin-openapi document object model with extra
// metadata so that the template can be entirely data driven
type Spec struct {
	ValidationContext context.Context // Required parameter for validation functions
	Model             openapi3.T      // Document object model we are building
}

func (s *Spec) ToBytes() ([]byte, error) {
	data, err := yaml.Marshal(s.Model)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (s *Spec) ToYAML() (string, error) {
	data, err := s.ToBytes()
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// SpecBuilder allows specifying different static analysis behaviors to that the
// framework can target any extant API.
type SpecBuilder struct {
	spec      *Spec
	logger    loggerFunc
	Generator *openapi3gen.Generator
}

func (b *SpecBuilder) BuildFromModel(logger loggerFunc) (*Spec, error) {
	b.logger = logger
	b.Generator = openapi3gen.NewGenerator(openapi3gen.UseAllExportedFields())
	// TODO: Eventually may need to support multiple OpenAPI versions, but pushing
	// that off for now.
	b.spec = &Spec{
		ValidationContext: context.Background(),
		Model: openapi3.T{
			OpenAPI: "3.0.3",
		},
	}

	if err := b.buildInfo(); err != nil {
		return nil, err
	}

	if err := b.buildSecurity(); err != nil {
		return nil, err
	}

	if err := b.buildServers(); err != nil {
		return nil, err
	}

	if err := b.buildTags(); err != nil {
		return nil, err
	}

	if err := b.buildComponentsAndPaths(); err != nil {
		return nil, err
	}

	if err := b.spec.Model.Validate(b.spec.ValidationContext); err != nil {
		return nil, err
	}

	return b.spec, nil
}

// buildInfo builds the Info field
func (b *SpecBuilder) buildInfo() error {
	if b.spec.Model.Info == nil {
		b.spec.Model.Info = &openapi3.Info{
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
	}
	return nil
}

// buildSecurity builds the Security field
func (b *SpecBuilder) buildSecurity() error {
	if b.spec.Model.Security == nil {
		b.spec.Model.Security = *openapi3.NewSecurityRequirements()
		b.spec.Model.Security.With(openapi3.SecurityRequirement{
			"X-Nomad-Token": {},
		})
	}
	return nil
}

// buildServers builds the Servers field
func (b *SpecBuilder) buildServers() error {
	if b.spec.Model.Servers == nil {
		b.spec.Model.Servers = openapi3.Servers{
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
	}
	return nil
}

// buildTags builds the Tags field
func (b *SpecBuilder) buildTags() error {
	if b.spec.Model.Tags == nil {
		b.spec.Model.Tags = openapi3.Tags{}
	}
	return nil
}

// buildComponentsAndPaths builds the Components object graph by parsing the model.
func (b *SpecBuilder) buildComponentsAndPaths() error {
	v1API := v1api{}
	b.spec.Model.Components = openapi3.NewComponents()

	b.spec.Model.Components.RequestBodies = openapi3.RequestBodies{}
	b.spec.Model.Components.Parameters = openapi3.ParametersMap{}
	b.spec.Model.Components.Responses = openapi3.NewResponses()
	b.spec.Model.Components.Schemas = openapi3.Schemas{}
	b.spec.Model.Components.Headers = openapi3.Headers{}
	b.spec.Model.Components.SecuritySchemes = openapi3.SecuritySchemes{}

	err := b.buildGraphFromModel(v1API)
	if err != nil {
		return err
	}

	b.resolveRefPaths()

	return nil
}

func (b *SpecBuilder) buildGraphFromModel(api v1api) error {
	paths := openapi3.Paths{}

	for _, path := range api.GetPaths() {
		if len(path.Template) < 1 {
			err := fmt.Errorf("invalid Path spec: %#v", path)
			b.logger(err)
			return err
		}

		pathItem := &openapi3.PathItem{}

		for _, op := range path.Operations {
			var err error
			var requestBodyRef *openapi3.RequestBodyRef
			var responses openapi3.Responses

			if op.RequestBody != nil {
				requestBodyRef, err = b.adaptRequestBody(op.RequestBody)
				if err != nil {
					err = fmt.Errorf("error %s: unable to adapt request body for %#v", err, op.RequestBody)
					b.logger(err)
					return err
				}
				if requestBodyRef != nil {
					requestBodyRef = &openapi3.RequestBodyRef{
						Ref:   "",
						Value: requestBodyRef.Value,
					}
				}
			}
			if responses, err = b.adaptResponses(op.Responses); err != nil {
				err = fmt.Errorf("error %s: unable to adapt Operation responses for %#v", err, op)
				b.logger("unable to adapt Operation responses", err, path, op)
				return err
			}

			operation := &openapi3.Operation{
				Tags:        op.Tags,
				Summary:     op.Summary,
				Description: op.Description,
				OperationID: op.Description,
				Parameters:  b.adaptParameters(op.Parameters),
				RequestBody: requestBodyRef,
				Responses:   responses,
			}

			// Add security if required
			security := b.adaptSecurityRequirements(op.Parameters)
			if security != nil {
				operation.Security = security
			}

			switch op.Method {
			case http.MethodGet:
				pathItem.Get = operation
			case http.MethodPost:
				pathItem.Post = operation
			case http.MethodDelete:
				pathItem.Delete = operation
			}
		}
		paths[path.Template] = pathItem
	}

	b.spec.Model.Paths = paths

	return nil
}

func (b *SpecBuilder) adaptParameters(params []*Parameter) openapi3.Parameters {
	parameters := openapi3.Parameters{}

	for _, param := range params {
		if existing, ok := b.spec.Model.Components.Parameters[param.Id]; ok {
			parameters = append(parameters, existing)
		} else {
			parameter := b.addParameter(param)
			parameters = append(parameters, parameter)
		}
	}

	return parameters
}

func (b *SpecBuilder) addParameter(param *Parameter) *openapi3.ParameterRef {
	if existing, ok := b.spec.Model.Components.Parameters[param.Id]; ok {
		return existing
	}

	parameter := &openapi3.ParameterRef{
		Ref: "",
		Value: &openapi3.Parameter{
			Name:        param.Name,
			Description: param.Description,
			Schema: openapi3.NewSchemaRef("", &openapi3.Schema{
				Type: param.SchemaType,
			}),
			In:       param.In,
			Required: param.Required,
		},
	}

	b.spec.Model.Components.Parameters[param.Id] = parameter

	return &openapi3.ParameterRef{
		Ref:   fmt.Sprintf("#/components/parameters/%s", param.Id),
		Value: parameter.Value,
	}
}

func (b *SpecBuilder) adaptRequestBody(requestBodyModel *RequestBody) (*openapi3.RequestBodyRef, error) {
	if existing, ok := b.spec.Model.Components.RequestBodies[requestBodyModel.Model.Name()]; ok {
		return existing, nil
	}

	requestBody := openapi3.NewRequestBody()
	schemaRef, err := b.getOrCreateSchemaRef(requestBodyModel.Model)
	if err != nil {
		return nil, err
	}

	requestBody.Required = true
	requestBody.Content = openapi3.NewContentWithSchemaRef(&openapi3.SchemaRef{
		Ref:   fmt.Sprintf("#/components/schemas/%s", requestBodyModel.Model.Name()),
		Value: schemaRef.Value,
	}, []string{"application/json"})

	b.spec.Model.Components.RequestBodies[requestBodyModel.Model.Name()] = &openapi3.RequestBodyRef{
		Ref:   "",
		Value: requestBody,
	}

	return b.spec.Model.Components.RequestBodies[requestBodyModel.Model.Name()], nil
}

func (b *SpecBuilder) adaptResponses(configs []*ResponseConfig) (openapi3.Responses, error) {
	responses := openapi3.Responses{}
	var response *openapi3.ResponseRef
	var ok bool
	for _, cfg := range configs {
		// if it isn't the global map, add it
		if response, ok = b.spec.Model.Components.Responses[cfg.Response.Name]; !ok {
			response = &openapi3.ResponseRef{
				Ref: "",
				Value: &openapi3.Response{
					Description: &cfg.Response.Description,
					Headers:     b.adaptHeaders(cfg.Headers),
				},
			}

			if cfg.Content != nil {
				schemaRef, err := b.getOrCreateSchemaRef(cfg.Content.Model)
				if err != nil {
					b.logger("unable to AdaptResponse for", cfg, err)
					return nil, err
				}

				switch cfg.Content.SchemaType {
				case objectSchema:
					response.Value.Content = openapi3.NewContentWithSchemaRef(&openapi3.SchemaRef{
						Ref:   fmt.Sprintf("#/components/schemas/%s", cfg.Content.Model.Name()),
						Value: schemaRef.Value,
					}, []string{"application/json"})
				case arraySchema:
					response.Value.Content = openapi3.NewContentWithSchemaRef(openapi3.NewSchemaRef("", &openapi3.Schema{
						Type: arraySchema,
						Items: &openapi3.SchemaRef{
							Ref:   fmt.Sprintf("#/components/schemas/%s", cfg.Content.Model.Name()),
							Value: schemaRef.Value,
						},
					}), []string{"application/json"})
				}
			}
			// Now add to the global response map
			if _, ok = b.spec.Model.Components.Responses[cfg.Response.Name]; !ok {
				b.spec.Model.Components.Responses[cfg.Response.Name] = response
			}
		}
		// Now add a ref for the path
		responses[strconv.Itoa(cfg.Code)] = &openapi3.ResponseRef{
			Ref:   fmt.Sprintf("#/components/responses/%s", cfg.Response.Name),
			Value: response.Value,
		}
	}

	return responses, nil
}

func (b *SpecBuilder) adaptHeaders(headers []*ResponseHeader) openapi3.Headers {
	var ok bool
	var headerRef *openapi3.HeaderRef
	headerRefs := openapi3.Headers{}

	for _, header := range headers {
		if headerRef, ok = b.spec.Model.Components.Headers[header.Name]; !ok {
			headerRef = &openapi3.HeaderRef{
				Ref: "", //fmt.Sprintf("#/components/parameters/%s", header.Name),
				Value: &openapi3.Header{
					Parameter: openapi3.Parameter{
						Name:        header.Name,
						Description: header.Description,
						Schema:      openapi3.NewSchemaRef("", &openapi3.Schema{Type: header.SchemaType}),
					},
				},
			}
		}

		headerRefs[header.Name] = headerRef
	}

	return headerRefs
}

// getOrCreateSchemaRef creates a SchemaRef for a Request or Response content. It
// adds all referenced schemas to the schemas map, but DOES NOT add the request
// or response schema. The callers are responsible for adding the root to the
// correct map.
func (b *SpecBuilder) getOrCreateSchemaRef(model reflect.Type) (*openapi3.SchemaRef, error) {
	var ok bool
	var err error
	var ref *openapi3.SchemaRef
	// if it doesn't exist generate
	if ref, ok = b.spec.Model.Components.Schemas[model.Name()]; !ok {
		ref, err = b.Generator.GenerateSchemaRef(model)
		if err != nil {
			return nil, err
		}
	}

	// Add all generated schema refs if not already added
	for schemaRef, _ := range b.Generator.SchemaRefs {
		if b.isBasic(schemaRef.Ref) {
			continue
		}
		if len(schemaRef.Ref) > 0 && !strings.Contains(schemaRef.Ref, "#/components/schemas") {
			if _, ok = b.spec.Model.Components.Schemas[schemaRef.Ref]; !ok {
				b.spec.Model.Components.Schemas[schemaRef.Ref] = openapi3.NewSchemaRef("", schemaRef.Value)
			}
		}
	}

	return ref, nil
}

func (b *SpecBuilder) isBasic(typ string) bool {
	return typ == "" || typ == "integer" || typ == "number" || typ == "string" || typ == "boolean"
}

func (b *SpecBuilder) resolveRefPaths() {
	for _, schemaRef := range b.spec.Model.Components.Schemas {
		// Next make sure the refs point to other schemas, if not already done.
		for _, property := range schemaRef.Value.Properties {
			if strings.Contains(property.Ref, "#/components/schemas/") {
				continue
			} else if b.isBasic(property.Value.Type) {
				property.Ref = ""
			} else if property.Value.Type == "array" {
				if b.isBasic(property.Value.Items.Value.Type) {
					property.Value.Items.Ref = ""
				} else {
					if !strings.Contains(property.Value.Items.Ref, "#/components/schemas") {
						property.Value.Items.Ref = fmt.Sprintf("#/components/schemas/%s", property.Value.Items.Ref)
					}
				}
			} else if property.Value.AdditionalProperties != nil {
				property.Ref = ""
				if !b.isBasic(property.Value.AdditionalProperties.Value.Type) {
					if len(property.Value.AdditionalProperties.Ref) > 1 {
						if !strings.Contains(property.Value.AdditionalProperties.Ref, "#/components/schemas") {
							property.Value.AdditionalProperties.Ref = fmt.Sprintf("#/components/schemas/%s", property.Value.AdditionalProperties.Ref)
						}
					}
				}
			} else {
				if !strings.Contains(property.Ref, "#/components/schemas") {
					property.Ref = fmt.Sprintf("#/components/schemas/%s", property.Ref)
				}
			}
		}
	}
}

func (b *SpecBuilder) adaptSecurityRequirements(parameters []*Parameter) *openapi3.SecurityRequirements {
	var tokenParam *Parameter
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
				In:   tokenParam.In,
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
