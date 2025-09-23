// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package hcl

import (
	"reflect"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclparse"
)

type Parser struct {
	parser  *hclparse.Parser
	decoder *gohcl.Decoder
}

// NewParser returns a new Parser instance with an empty gohcl.Decoder that can
// be used for parsing and decoding HCL files into Go structs.
func NewParser() *Parser {
	return &Parser{
		decoder: &gohcl.Decoder{},
		parser:  hclparse.NewParser(),
	}
}

func (p *Parser) AddExpressionDecoder(typ reflect.Type, fn gohcl.ExpressionDecoderFunc) {
	p.decoder.RegisterExpressionDecoder(typ, fn)
}

func (p *Parser) Parse(src []byte, dst any, filename string) hcl.Diagnostics {

	hclFile, parseDiag := p.parser.ParseHCL(src, filename)

	if parseDiag.HasErrors() {
		return parseDiag
	}

	decodeDiag := p.decoder.DecodeBody(hclFile.Body, nil, dst)
	return decodeDiag
}
