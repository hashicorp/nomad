package resources

import (
	"fmt"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"reflect"

	"github.com/hashicorp/go-multierror"
	hcl "github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/zclconf/go-cty/cty/gocty"
)

// Resource is a custom resource that users can configure to expose custom capabilities
// available per client.
type Resource struct {
	Name  string `hcl:"name,label"`
	Range *Range `hcl:"range,block,optional"`
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

func (r *Range) Validate(iface interface{}) error {
	val, ok := iface.(int)
	if !ok {
		return fmt.Errorf("invalid resource config: value %#v cannot be cast to int", iface)
	}
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

	resourceContent, resourceDiags := file.Body.Content(&hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{
				Type:       "resource",
				LabelNames: []string{"name"},
			},
		},
	})

	diags = append(diags, resourceDiags...)
	if diags.HasErrors() {
		return nil, diags
	}

	result := &Resource{}

	if len(resourceContent.Blocks) == 0 {
		return nil, fmt.Errorf("no resource found")
	}

	for _, block := range resourceContent.Blocks {
		if block.Type != "resource" {
			continue
		}

		result.Name = block.Labels[0]

		diags = append(diags, parseRange(block, result)...)

		// Add parsing logic for all custom resource block types here, then trap
		// parsing errors once at the end of the loop.

		if diags.HasErrors() {
			return nil, diags
		}
	}

	return result, nil
}

func parseRange(block *hcl.Block, result *Resource) hcl.Diagnostics {
	var diags hcl.Diagnostics

	rangeContent, _, rangeDiags := block.Body.PartialContent(&hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{
				Type: "range",
			},
		},
	})

	diags = append(diags, rangeDiags...)
	if diags.HasErrors() {
		return diags
	}

	for _, rangeBlock := range rangeContent.Blocks {
		if rangeBlock.Type != "range" {
			return append(diags, &hcl.Diagnostic{
				Severity: 0,
				Summary:  "range parse error",
				Detail:   fmt.Sprintf("invalid block type: %s", rangeBlock.Type),
				Subject:  &rangeBlock.TypeRange,
				Context:  &block.TypeRange,
			})
		}

		result.Range = &Range{}

		rangeBlockContent, _, rangeBlockDiags := rangeBlock.Body.PartialContent(&hcl.BodySchema{
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
		})

		diags = append(diags, rangeBlockDiags...)
		if diags.HasErrors() {
			return diags
		}

		for _, attribute := range rangeBlockContent.Attributes {
			val, valDiags := attribute.Expr.Value(nil)
			diags = append(diags, valDiags...)
			if diags.HasErrors() {
				return diags
			}

			switch attribute.Name {
			case "upper":
				gocty.FromCtyValue(val, &result.Range.Upper)
			case "lower":
				gocty.FromCtyValue(val, &result.Range.Lower)
			default:
				diags = append(diags, &hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  "unsupported attribute",
					Detail:   fmt.Sprintf("attribute %q is not supported", attribute.Name),
					Subject:  &attribute.Range,
					Context:  &rangeBlock.TypeRange,
				})
			}
		}
	}

	return diags
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
