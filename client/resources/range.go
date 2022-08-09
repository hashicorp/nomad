package resources

import (
	"fmt"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty/gocty"
)

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

func maybeParseRange(block *hcl.Block, result *Resource) hcl.Diagnostics {
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
		// TODO: This might need to be moved up to the Parse function level once we have another resource type.
		if rangeBlock.Type != "range" {
			return append(diags, &hcl.Diagnostic{
				Severity: 0,
				Summary:  "range parse error",
				Detail:   fmt.Sprintf("invalid block type: %s", rangeBlock.Type),
				Subject:  &rangeBlock.TypeRange,
				Context:  &block.TypeRange,
			})
		}

		// TODO: This logic might be best on the Resource type. Resource then can built itself.
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
				err := gocty.FromCtyValue(val, &result.Range.Upper)
				if err != nil {
					diags = appendDiag(diags, attribute, rangeBlock, "type conversion error", fmt.Sprintf("attribute %q could not be converted from cty value %#v", attribute.Name, val))
				}
			case "lower":
				err := gocty.FromCtyValue(val, &result.Range.Lower)
				if err != nil {
					diags = appendDiag(diags, attribute, rangeBlock, "type conversion error", fmt.Sprintf("attribute %q could not be converted from cty value %#v", attribute.Name, val))
				}
			default:
				diags = appendDiag(diags, attribute, rangeBlock, "unsupported attribute", fmt.Sprintf("attribute %q is not supported", attribute.Name))
			}
		}
	}

	return diags
}
