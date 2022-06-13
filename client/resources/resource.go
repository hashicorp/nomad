package resources

import (
	"fmt"
	"github.com/hashicorp/hcl/v2/ext/dynblock"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/nomad/jobspec2/hclutil"
	"reflect"

	"github.com/hashicorp/go-multierror"
	hcl "github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
)

// Resource is a custom resource that users can configure to expose custom capabilities
// available per client.
type Resource struct {
	Name  string `hcl:"name,label"`
	Range *Range `hcl:"range,optional"`
}

func (r *Resource) ValidateConfig() error {
	mErr := new(multierror.Error)

	if r.Range != nil {
		if err := r.Range.validateConfig(); err != nil {
			mErr = multierror.Append(mErr, fmt.Errorf("invalid config: resource %s of type range returned error - %s", r.Name, err.Error()))
		}
	}

	return mErr.ErrorOrNil()
}

// Range is a ResourceType that ensures resource configuration contains an integer
// value within the allowable upper and lower bounds.
type Range struct {
	Upper int `hcl:"upper"`
	Lower int `hcl:"lower"`
}

func (r *Range) Validate(val int) error {
	if val < r.Lower {
		return fmt.Errorf("invalid resource config: value %d cannot be less than lower bound %d", val, r.Lower)
	}

	if val > r.Upper {
		return fmt.Errorf("invalid resource config: value %d cannot be greater than upper bound %d", val, r.Upper)
	}

	return nil
}

func (r *Range) validateConfig() error {
	mErr := new(multierror.Error)

	if r.Lower > r.Upper {
		mErr = multierror.Append(mErr, fmt.Errorf("lower bound %d is greater than upper bound %d", r.Lower, r.Upper))
	}

	return mErr.ErrorOrNil()
}

var hclDecoder *gohcl.Decoder

func init() {
	hclDecoder := &gohcl.Decoder{}
	hclDecoder.RegisterBlockDecoder(reflect.TypeOf(Resource{}), decodeCustomResource)
}

// Parse parses the resource spec from the given string.
func Parse(tmplContent string, filename string) (*Resource, error) {
	file, diags := hclsyntax.ParseConfig([]byte(tmplContent), filename, hcl.Pos{Byte: 0, Line: 1, Column: 1})
	if diags.HasErrors() {
		return nil, fmt.Errorf("error parsing: %s", diags)
	}

	content, schemaDiags := file.Body.Content(&hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{
				Type:       "resource",
				LabelNames: []string{"name"},
			},
			{
				Type: "range",
			},
		},
	})

	diags = append(diags, schemaDiags...)
	if diags.HasErrors() {
		return nil, diags
	}

	result := &Resource{}
	decoder := &gohcl.Decoder{}
	for _, block := range content.Blocks {
		if block.Type != "resource" {
			continue
		}
		body := hclutil.BlocksAsAttrs(block.Body)
		body = dynblock.Expand(body, nil)

		result.Name = block.Labels[0]

		resourceContent, resourceBody, resourceDiags := body.PartialContent(&hcl.BodySchema{
			Attributes: []hcl.AttributeSchema{
				{
					Name:     "lower",
					Required: true,
				},
				{
					Name:     "upper",
					Required: true,
				},
			},
			Blocks: []hcl.BlockHeaderSchema{
				{
					Type: "range",
				},
			},
		})
		diags = append(diags, resourceDiags...)
		if diags.HasErrors() {
			return nil, diags
		}

		// diags = append(diags, decoder.DecodeBody(resourceContent, nil, result.Range)...)

		for _, resourceBlock := range resourceContent.Blocks {
			switch resourceBlock.Type {
			case "range":
				result.Range = &Range{}
				decodeDiags := decoder.DecodeBody(resourceBody, nil, result.Range)
				diags = append(diags, decodeDiags...)
				if diags.HasErrors() {
					return nil, diags
				}
			default:
				// Shouldn't get here, because the above cases are exhaustive for
				// our test file schema.
				panic(fmt.Sprintf("unsupported block type %q", block.Type))
			}
		}

	}

	return result, nil
}

func decodeCustomResource(body hcl.Body, ctx *hcl.EvalContext, val interface{}) hcl.Diagnostics {
	//t := val.(*Resource)
	var diags hcl.Diagnostics

	b, remain, moreDiags := body.PartialContent(&hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{Type: "resource", LabelNames: []string{"name"}},
		},
	})

	diags = append(diags, moreDiags...)

	if len(b.Blocks) == 0 {
		return nil
	}

	decoder := &gohcl.Decoder{}
	diags = append(diags, decoder.DecodeBody(remain, ctx, val)...)

	return diags
}
