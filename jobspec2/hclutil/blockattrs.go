package hclutil

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
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

		na := attr.AsHCLAttribute()
		na.Expr = attrExpr(attr.Expr)
		attrs[name] = na
	}

	for _, blocks := range blocksByType(body.Blocks) {
		if _, hidden := b.hiddenBlocks[blocks[0].Type]; hidden {
			continue
		}

		b := blocks[0]
		attr := &hcls.Attribute{
			Name:        b.Type,
			NameRange:   b.TypeRange,
			EqualsRange: b.OpenBraceRange,
			SrcRange:    b.Body.SrcRange,
			Expr:        blocksToExpr(blocks),
		}

		attrs[blocks[0].Type] = attr.AsHCLAttribute()
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

func blocksByType(blocks hcls.Blocks) map[string]hcls.Blocks {
	r := map[string]hclsyntax.Blocks{}
	for _, b := range blocks {
		r[b.Type] = append(r[b.Type], b)
	}
	return r
}

func blocksToExpr(blocks hcls.Blocks) hcls.Expression {
	if len(blocks) == 0 {
		panic("unexpected empty blocks")
	}

	exprs := make([]hcls.Expression, len(blocks))
	for i, b := range blocks {
		exprs[i] = blockToExpr(b)
	}

	last := blocks[len(blocks)-1]
	return &hcls.TupleConsExpr{
		Exprs: exprs,

		SrcRange:  hcl.RangeBetween(blocks[0].OpenBraceRange, last.CloseBraceRange),
		OpenRange: blocks[0].OpenBraceRange,
	}
}

func blockToExpr(b *hcls.Block) hcls.Expression {
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
			ValueExpr: attrExpr(attr.Expr),
		})
	}

	for _, blocks := range blocksByType(b.Body.Blocks) {
		keyExpr := &hcls.ScopeTraversalExpr{
			Traversal: hcl.Traversal{
				hcl.TraverseRoot{
					Name:     blocks[0].Type,
					SrcRange: blocks[0].TypeRange,
				},
			},
			SrcRange: blocks[0].TypeRange,
		}
		key := &hcls.ObjectConsKeyExpr{
			Wrapped: keyExpr,
		}
		item := hcls.ObjectConsItem{
			KeyExpr:   key,
			ValueExpr: blocksToExpr(blocks),
		}

		items = append(items, item)
	}

	v := &hcls.ObjectConsExpr{
		Items: items,
	}

	// Create nested maps, with the labels as keys.
	// Starts wrapping from most inner label to outer
	for i := len(b.Labels) - 1; i >= 0; i-- {
		keyExpr := &hcls.ScopeTraversalExpr{
			Traversal: hcl.Traversal{
				hcl.TraverseRoot{
					Name:     b.Labels[i],
					SrcRange: b.LabelRanges[i],
				},
			},
			SrcRange: b.LabelRanges[i],
		}
		key := &hcls.ObjectConsKeyExpr{
			Wrapped: keyExpr,
		}
		item := hcls.ObjectConsItem{
			KeyExpr: key,
			ValueExpr: &hcls.TupleConsExpr{
				Exprs: []hcls.Expression{v},
			},
		}

		v = &hcls.ObjectConsExpr{
			Items: []hcls.ObjectConsItem{item},
		}

	}
	return v
}

func attrExpr(expr hcls.Expression) hcls.Expression {
	if _, ok := expr.(*hcls.ObjectConsExpr); ok {
		return &hcls.TupleConsExpr{
			Exprs:     []hcls.Expression{expr},
			SrcRange:  expr.Range(),
			OpenRange: expr.StartRange(),
		}
	}

	return expr
}
