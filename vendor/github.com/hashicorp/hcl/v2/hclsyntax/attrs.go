package hclsyntax

import "github.com/hashicorp/hcl/v2"

func convertToAttribute(b *Block) *Attribute {
	items := []ObjectConsItem{}

	for _, attr := range b.Body.Attributes {
		if _, hidden := b.Body.hiddenAttrs[attr.Name]; hidden {
			continue
		}

		keyExpr := &ScopeTraversalExpr{
			Traversal: hcl.Traversal{
				hcl.TraverseRoot{
					Name:     attr.Name,
					SrcRange: attr.NameRange,
				},
			},
			SrcRange: attr.NameRange,
		}
		key := &ObjectConsKeyExpr{
			Wrapped: keyExpr,
		}

		items = append(items, ObjectConsItem{
			KeyExpr:   key,
			ValueExpr: attr.Expr,
		})
	}

	for _, block := range b.Body.Blocks {
		if _, hidden := b.Body.hiddenBlocks[block.Type]; hidden {
			continue
		}

		keyExpr := &ScopeTraversalExpr{
			Traversal: hcl.Traversal{
				hcl.TraverseRoot{
					Name:     block.Type,
					SrcRange: block.TypeRange,
				},
			},
			SrcRange: block.TypeRange,
		}
		key := &ObjectConsKeyExpr{
			Wrapped: keyExpr,
		}
		valExpr := convertToAttribute(block).Expr
		items = append(items, ObjectConsItem{
			KeyExpr:   key,
			ValueExpr: valExpr,
		})
	}

	attr := &Attribute{
		Name:        b.Type,
		NameRange:   b.TypeRange,
		EqualsRange: b.OpenBraceRange,
		SrcRange:    b.Body.SrcRange,
		Expr: &ObjectConsExpr{
			Items: items,
		},
	}

	return attr
}
