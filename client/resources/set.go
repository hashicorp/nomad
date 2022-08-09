package resources

import (
	"errors"
	"fmt"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty/gocty"
)

type Set struct {
	Members []string `hcl:"members"`
}

func (s *Set) Validate(iface interface{}) error {
	val, ok := iface.(string)
	if !ok {
		return fmt.Errorf("invalid resource config: value %#v cannot be cast to string", iface)
	}

	for _, member := range s.Members {
		if member == val {
			return nil
		}
	}

	return fmt.Errorf("invalid resource config: value %s is not a member of set", val)
}

func (s *Set) validateConfig() error {
	mErr := new(multierror.Error)

	if len(s.Members) < 1 {
		mErr = multierror.Append(mErr, errors.New("set has no members"))
	}

	entries := map[string]struct{}{}

	for _, member := range s.Members {
		if _, ok := entries[member]; ok {
			mErr = multierror.Append(mErr, fmt.Errorf("member %s defined more than once", member))
		}
		entries[member] = struct{}{}
	}

	return mErr.ErrorOrNil()
}

func maybeParseSet(block *hcl.Block, result *Resource) hcl.Diagnostics {
	var diags hcl.Diagnostics

	content, _, contentDiags := block.Body.PartialContent(&hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{
				Type: "set",
			},
		},
	})

	diags = append(diags, contentDiags...)
	if diags.HasErrors() {
		return diags
	}

	for _, setBlock := range content.Blocks {
		// TODO: This might need to be moved up to the Parse function level once we have another resource type.
		if setBlock.Type != "set" {
			return append(diags, &hcl.Diagnostic{
				Severity: 0,
				Summary:  "set parse error",
				Detail:   fmt.Sprintf("invalid block type: %s", setBlock.Type),
				Subject:  &setBlock.TypeRange,
				Context:  &block.TypeRange,
			})
		}

		// TODO: This logic might be best on the Resource type. Resource then can built itself.
		result.Set = &Set{}

		setBlockContent, _, setBlockDiags := setBlock.Body.PartialContent(&hcl.BodySchema{
			Attributes: []hcl.AttributeSchema{
				{
					Name:     "members",
					Required: true,
				},
			},
		})

		diags = append(diags, setBlockDiags...)
		if diags.HasErrors() {
			return diags
		}

		for _, attribute := range setBlockContent.Attributes {
			val, valDiags := attribute.Expr.Value(nil)
			diags = append(diags, valDiags...)
			if diags.HasErrors() {
				return diags
			}

			switch attribute.Name {
			case "members":
				members := val.AsValueSlice()
				for _, member := range members {
					var m string
					err := gocty.FromCtyValue(member, &m)
					if err != nil {
						diags = appendDiag(diags, attribute, setBlock, "type conversion error", fmt.Sprintf("attribute %q could not be converted from cty value %#v", attribute.Name, member))
					} else {
						result.Set.Members = append(result.Set.Members, m)
					}
				}
			default:
				diags = appendDiag(diags, attribute, setBlock, "unsupported attribute", fmt.Sprintf("attribute %q is not supported", attribute.Name))
			}
		}
	}

	return diags
}
