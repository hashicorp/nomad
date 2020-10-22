package hclutil

import (
	"github.com/hashicorp/hcl/v2"
	hcls "github.com/hashicorp/hcl/v2/hclsyntax"
)

// BlocksAsAttrs rewrites the hcl.Body so that hcl blocks are treated as
// attributes when schema is unknown.
//
// This conversion is necessary for parsing task driver configs, as they can be
// arbitrary nested without pre-defined schema.
//
// More concretely, it changes the following:
//
// ```
// config {
//   meta { ... }
// }
// ```

// to
//
// ```
// config {
//   meta = { ... } # <- attribute now
// }
// ```
func BlocksAsAttrs(body hcl.Body) hcl.Body {
	if hclb, ok := body.(*hcls.Body); ok {
		return &blockAttrs{body: hclb}
	}
	return body
}

type blockAttrs struct {
	body hcl.Body

	hiddenAttrs  map[string]struct{}
	hiddenBlocks map[string]struct{}
}

func (b *blockAttrs) Content(schema *hcl.BodySchema) (*hcl.BodyContent, hcl.Diagnostics) {
	bc, diags := b.body.Content(schema)
	bc.Blocks = expandBlocks(bc.Blocks)
	return bc, diags
}
func (b *blockAttrs) PartialContent(schema *hcl.BodySchema) (*hcl.BodyContent, hcl.Body, hcl.Diagnostics) {
	bc, remainBody, diags := b.body.PartialContent(schema)
	bc.Blocks = expandBlocks(bc.Blocks)

	remain := &blockAttrs{
		body:         remainBody,
		hiddenAttrs:  map[string]struct{}{},
		hiddenBlocks: map[string]struct{}{},
	}
	for name := range b.hiddenAttrs {
		remain.hiddenAttrs[name] = struct{}{}
	}
	for typeName := range b.hiddenBlocks {
		remain.hiddenBlocks[typeName] = struct{}{}
	}
	for _, attrS := range schema.Attributes {
		remain.hiddenAttrs[attrS.Name] = struct{}{}
	}
	for _, blockS := range schema.Blocks {
		remain.hiddenBlocks[blockS.Type] = struct{}{}
	}

	return bc, remain, diags
}

func (b *blockAttrs) JustAttributes() (hcl.Attributes, hcl.Diagnostics) {
	body, ok := b.body.(*hcls.Body)
	if !ok {
		return b.body.JustAttributes()
	}

	attrs := make(hcl.Attributes)
	var diags hcl.Diagnostics

	if body.Attributes == nil && len(body.Blocks) == 0 {
		return attrs, diags
	}

	for name, attr := range body.Attributes {
		if _, hidden := b.hiddenAttrs[name]; hidden {
			continue
		}
		attrs[name] = attr.AsHCLAttribute()
	}

	for _, blockS := range body.Blocks {
		if _, hidden := b.hiddenBlocks[blockS.Type]; hidden {
			continue
		}

		attrs[blockS.Type] = convertToAttribute(blockS).AsHCLAttribute()
	}

	return attrs, diags
}

func (b *blockAttrs) MissingItemRange() hcl.Range {
	return b.body.MissingItemRange()
}

func expandBlocks(blocks hcl.Blocks) hcl.Blocks {
	if len(blocks) == 0 {
		return blocks
	}

	r := make([]*hcl.Block, len(blocks))
	for i, b := range blocks {
		nb := *b
		nb.Body = BlocksAsAttrs(b.Body)
		r[i] = &nb
	}
	return r
}

func convertToAttribute(b *hcls.Block) *hcls.Attribute {
	items := []hcls.ObjectConsItem{}

	for _, attr := range b.Body.Attributes {
		keyExpr := &hcls.ScopeTraversalExpr{
			Traversal: hcl.Traversal{
				hcl.TraverseRoot{
					Name:     attr.Name,
					SrcRange: attr.NameRange,
				},
			},
			SrcRange: attr.NameRange,
		}
		key := &hcls.ObjectConsKeyExpr{
			Wrapped: keyExpr,
		}

		items = append(items, hcls.ObjectConsItem{
			KeyExpr:   key,
			ValueExpr: attr.Expr,
		})
	}

	for _, block := range b.Body.Blocks {
		keyExpr := &hcls.ScopeTraversalExpr{
			Traversal: hcl.Traversal{
				hcl.TraverseRoot{
					Name:     block.Type,
					SrcRange: block.TypeRange,
				},
			},
			SrcRange: block.TypeRange,
		}
		key := &hcls.ObjectConsKeyExpr{
			Wrapped: keyExpr,
		}
		valExpr := convertToAttribute(block).Expr
		items = append(items, hcls.ObjectConsItem{
			KeyExpr:   key,
			ValueExpr: valExpr,
		})
	}

	attr := &hcls.Attribute{
		Name:        b.Type,
		NameRange:   b.TypeRange,
		EqualsRange: b.OpenBraceRange,
		SrcRange:    b.Body.SrcRange,
		Expr: &hcls.ObjectConsExpr{
			Items: items,
		},
	}

	return attr
}
