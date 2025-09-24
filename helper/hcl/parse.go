// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package hcl

import (
	"reflect"
	"time"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclparse"
)

type Parser struct {
	parser  *hclparse.Parser
	decoder *gohcl.Decoder
}

// NewParser returns a new Parser instance which supports decoding time.Duration
// parameters by default.
func NewParser() *Parser {

	// Create our base decoder, so we can register custom decoders on it.
	decoder := &gohcl.Decoder{}

	// Register default custom decoders here which currently only includes
	// time.Duration parsing.
	dur := time.Duration(0)
	decoder.RegisterExpressionDecoder(reflect.TypeOf(dur), DecodeDuration)
	decoder.RegisterExpressionDecoder(reflect.TypeOf(&dur), DecodeDuration)

	return &Parser{
		decoder: decoder,
		parser:  hclparse.NewParser(),
	}
}

func (p *Parser) Parse(src []byte, dst any, filename string) hcl.Diagnostics {

	hclFile, parseDiag := p.parser.ParseHCL(src, filename)

	if parseDiag.HasErrors() {
		return parseDiag
	}

	decodeDiag := p.decoder.DecodeBody(hclFile.Body, nil, dst)
	return decodeDiag
}
