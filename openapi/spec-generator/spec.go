package main

import (
	"context"
	"fmt"
	openapi3 "github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3gen"
	"github.com/ghodss/yaml"
	"net/http"
	"reflect"
	"strconv"
	"strings"
)

// Spec wraps a kin-openapi document object model with a little bit of extra
// metadata so that the template can be entirely data driven
type Spec struct {
	ValidationContext context.Context // Required parameter for validation functions
	Model             openapi3.T      // Document object model we are building
}

func (s *Spec) ToYAML() (string, error) {
	data, err := yaml.Marshal(s.Model)
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
	Visitor   *PackageVisitor
	Generator *openapi3gen.Generator
}

// Build runs a default implementation to build and OpenAPI spec. Derived types
// may choose to override if they don't need to execute this full pipeline.
func (b *SpecBuilder) Build() (*Spec, error) {
	// TODO: Eventually may need to support multiple OpenAPI versions, but pushing
	// that off for now.
	b.spec = &Spec{
		ValidationContext: context.Background(),
		Model: openapi3.T{
			OpenAPI: "3.0.3",
		},
	}

	if err := b.BuildInfo(); err != nil {
		return nil, err
	}

	if err := b.BuildSecurity(); err != nil {
		return nil, err
	}

	if err := b.BuildServers(); err != nil {
		return nil, err
	}

	if err := b.BuildTags(); err != nil {
		return nil, err
	}

	if err := b.BuildComponents(); err != nil {
		return nil, err
	}

	if err := b.BuildPaths(); err != nil {
		return nil, err
	}

	if err := b.spec.Model.Validate(b.spec.ValidationContext); err != nil {
		return nil, err
	}

	return b.spec, nil
}

func (b *SpecBuilder) BuildFromModel() (*Spec, error) {
	b.Generator = openapi3gen.NewGenerator(openapi3gen.UseAllExportedFields())
	// TODO: Eventually may need to support multiple OpenAPI versions, but pushing
	// that off for now.
	b.spec = &Spec{
		ValidationContext: context.Background(),
		Model: openapi3.T{
			OpenAPI: "3.0.3",
		},
	}

	if err := b.BuildInfo(); err != nil {
		return nil, err
	}

	if err := b.BuildSecurity(); err != nil {
		return nil, err
	}

	if err := b.BuildServers(); err != nil {
		return nil, err
	}

	if err := b.BuildTags(); err != nil {
		return nil, err
	}

	if err := b.BuildComponentsFromModel(); err != nil {
		return nil, err
	}

	if err := b.spec.Model.Validate(b.spec.ValidationContext); err != nil {
		return nil, err
	}

	return b.spec, nil
}

// BuildInfo builds the Info field
func (b *SpecBuilder) BuildInfo() error {
	if b.spec.Model.Info == nil {
		b.spec.Model.Info = &openapi3.Info{
			Version: "1.1.0", // TODO: Schlep this dynamically from VersionInfo
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

// BuildSecurity builds the Security field
// TODO: Might be useful for interface, but might not need this for Nomad
func (b *SpecBuilder) BuildSecurity() error {
	if b.spec.Model.Security == nil {
		b.spec.Model.Security = *openapi3.NewSecurityRequirements()
	}
	return nil
}

// BuildServers builds the Servers field
// TODO: Might be useful for interface, but might not need this for Nomad
func (b *SpecBuilder) BuildServers() error {
	if b.spec.Model.Servers == nil {
		b.spec.Model.Servers = openapi3.Servers{}
	}
	return nil
}

// BuildTags builds the Tags field
// TODO: Might be useful for interface, but might not need this for Nomad
func (b *SpecBuilder) BuildTags() error {
	if b.spec.Model.Tags == nil {
		b.spec.Model.Tags = openapi3.Tags{}
	}
	return nil
}

// BuildComponents builds the Components field
// TODO: Might be useful for interface, but might not need this for Nomad
func (b *SpecBuilder) BuildComponents() error {
	b.spec.Model.Components = openapi3.NewComponents()

	visitor := *b.Visitor

	b.spec.Model.Components.Schemas = visitor.SchemaRefs()
	b.spec.Model.Components.Parameters = visitor.ParameterRefs()
	b.spec.Model.Components.Headers = visitor.HeaderRefs()
	b.spec.Model.Components.RequestBodies = visitor.RequestBodyRefs()
	b.spec.Model.Components.Callbacks = visitor.CallbackRefs()
	b.spec.Model.Components.Responses = visitor.ResponseRefs()
	b.spec.Model.Components.SecuritySchemes = openapi3.SecuritySchemes{}

	return nil
}

// BuildPaths builds the Paths field
func (b *SpecBuilder) BuildPaths() error {
	if b.spec.Model.Paths == nil {
		b.spec.Model.Paths = openapi3.Paths{}
	}

	visitor := *b.Visitor
	for _, adapter := range visitor.HandlerAdapters() {
		pathItem := &openapi3.PathItem{}

		b.spec.Model.Paths[adapter.GetPath()] = pathItem
	}

	return nil
}

func (b *SpecBuilder) BuildComponentsFromModel() error {
	v1API := V1API{}
	b.spec.Model.Components = openapi3.NewComponents()

	b.spec.Model.Components.RequestBodies = openapi3.RequestBodies{}
	b.spec.Model.Components.Parameters = openapi3.ParametersMap{}
	b.spec.Model.Components.Responses = openapi3.NewResponses()
	b.spec.Model.Components.Schemas = openapi3.Schemas{}
	b.spec.Model.Components.Headers = openapi3.Headers{}
	b.spec.Model.Components.SecuritySchemes = openapi3.SecuritySchemes{}

	b.spec.Model.Paths = b.BuildPathsFromModel(v1API)
	b.resolveRefPaths()

	return nil
}

func (b *SpecBuilder) BuildPathsFromModel(api V1API) openapi3.Paths {
	paths := openapi3.Paths{}

	for _, path := range api.GetPaths() {
		if len(path.Key) < 1 {
			b.logger("invalid PathItem spec", path)
			continue
		}

		pathItem := &openapi3.PathItem{}

		for _, op := range path.Operations {
			var err error
			var requestBodyRef *openapi3.RequestBodyRef
			var responses openapi3.Responses

			if op.RequestBody != nil {
				requestBodyRef, err = b.AdaptRequestBody(op.RequestBody)
				if err != nil {
					b.logger("unable to adapt request body for", op.RequestBody)
				}
			}
			if responses, err = b.AdaptResponses(op.Responses); err != nil {
				b.logger("unable to adapt Operation responses", path, op)
				continue
			}
			operation := &openapi3.Operation{
				Tags: op.Tags,
				// Optional short summary.
				Summary:     op.Summary,
				Description: op.Description,
				OperationID: op.Description,
				Parameters:  b.AdaptParameters(op.Parameters),
				RequestBody: requestBodyRef,
				Responses:   responses,
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
		paths[path.Key] = pathItem
	}

	return paths
}

func (b *SpecBuilder) AdaptParameters(params []*Parameter) openapi3.Parameters {
	parameters := openapi3.Parameters{}

	for _, param := range params {
		if existing, ok := b.spec.Model.Components.Parameters[param.Id]; ok {
			parameters = append(parameters, existing)
		} else {
			parameter := b.AddParameter(param)
			parameters = append(parameters, parameter)
		}
	}

	return parameters
}

func (b *SpecBuilder) AddParameter(param *Parameter) *openapi3.ParameterRef {
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

func (b *SpecBuilder) AdaptRequestBody(requestBodyModel *RequestBody) (*openapi3.RequestBodyRef, error) {
	requestBody := openapi3.NewRequestBody()
	schemaRef, err := b.GetOrCreateSchemaRef(requestBodyModel.Model)
	if err != nil {
		return nil, err
	}

	requestBody.Required = true
	requestBody.Content = openapi3.NewContentWithSchema(schemaRef.Value, []string{"application/json"})

	if _, ok := b.spec.Model.Components.RequestBodies[requestBodyModel.Model.Name()]; !ok {
		b.spec.Model.Components.RequestBodies[requestBodyModel.Model.Name()] = &openapi3.RequestBodyRef{
			Ref:   "",
			Value: requestBody,
		}
	}

	return &openapi3.RequestBodyRef{
		Ref:   "",
		Value: requestBody,
	}, nil
}

func (b *SpecBuilder) AdaptResponses(configs []*ResponseConfig) (openapi3.Responses, error) {
	responses := openapi3.Responses{}
	var response *openapi3.ResponseRef
	var ok bool
	for _, cfg := range configs {
		// if it isn't the global map, add it
		if response, ok = b.spec.Model.Components.Responses[cfg.Response.Id]; !ok {
			response = &openapi3.ResponseRef{
				Ref: "",
				Value: &openapi3.Response{
					Description: &cfg.Response.Description,
					Headers:     b.AdaptHeaders(cfg.Headers),
				},
			}

			if cfg.Content != nil {
				schemaRef, err := b.GetOrCreateSchemaRef(cfg.Content.Model)
				if err != nil {
					b.logger("unable to AdaptResponse for", cfg)
					return nil, err
				}

				response.Value.Content = openapi3.NewContentWithSchemaRef(schemaRef,
					[]string{"application/json"})
			}
			// Now add to the global response map
			b.spec.Model.Components.Responses[cfg.Response.Id] = response
		}
		// Now just add a ref for the path
		responses[strconv.Itoa(cfg.Code)] = &openapi3.ResponseRef{
			Ref:   fmt.Sprintf("#/components/responses/%s", cfg.Response.Id),
			Value: response.Value,
		}
	}

	return responses, nil
}

func (b *SpecBuilder) AdaptHeaders(hdrs []*Parameter) openapi3.Headers {
	var ok bool
	var headerRef *openapi3.HeaderRef
	headers := openapi3.Headers{}

	for _, hdr := range hdrs {
		if headerRef, ok = b.spec.Model.Components.Headers[hdr.Id]; !ok {
			var param *openapi3.ParameterRef
			if param, ok = b.spec.Model.Components.Parameters[hdr.Id]; !ok {
				param = b.AddParameter(hdr)
			}
			headerRef = &openapi3.HeaderRef{
				Ref: fmt.Sprintf("#/components/parameters/%s", hdr.Id),
				Value: &openapi3.Header{
					Parameter: *param.Value,
				},
			}
		}

		headers[hdr.Id] = headerRef
	}

	return headers
}

func (b *SpecBuilder) GetOrCreateSchemaRef(model reflect.Type) (*openapi3.SchemaRef, error) {
	var ok bool
	var err error
	var ref *openapi3.SchemaRef
	// if it doesn't exist generate and add it
	if ref, ok = b.spec.Model.Components.Schemas[model.Name()]; !ok {
		ref, err = b.Generator.GenerateSchemaRef(model)
		if err != nil {
			return nil, err
		}
		if !b.isBasic(ref.Ref) {
			b.spec.Model.Components.Schemas[ref.Ref] = ref
		}
	}

	// if adding the new ref generated new schemaRefs, add those
	for schemaRef, _ := range b.Generator.SchemaRefs {
		if len(schemaRef.Ref) > 0 && schemaRef.Ref != ref.Ref && !strings.Contains(schemaRef.Ref, "#/components/schemas") {
			if !b.isBasic(schemaRef.Ref) {
				if _, ok := b.spec.Model.Components.Schemas[schemaRef.Ref]; !ok {
					b.spec.Model.Components.Schemas[schemaRef.Ref] = openapi3.NewSchemaRef("", schemaRef.Value)
					if schemaRef.Value.Type == "array" {

					}
				}
			}
		}
	}

	return ref, nil
}

func (b *SpecBuilder) isBasic(typ string) bool {
	return typ == "" || strings.Contains(typ, "int") || typ == "number" || typ == "string" || strings.Contains(typ, "bool")
}

func (b *SpecBuilder) resolveRefPaths() {
	for _, schemaRef := range b.spec.Model.Components.Schemas {
		// Next make sure the refs point to other schemas, if not already done.
		for _, property := range schemaRef.Value.Properties {
			if b.isBasic(property.Value.Type) {
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
