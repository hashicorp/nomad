// Package hclsimple is a higher-level entry point for loading HCL
// configuration files directly into Go struct values in a single step.
//
// This package is more opinionated than the rest of the HCL API. See the
// documentation for function Decode for more information.
package hclsimple

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/json"
)

// Decode parses, decodes, and evaluates expressions in the given HCL source
// code, in a single step.
//
// The main HCL API is built to allow applications that need to decompose
// the processing steps into a pipeline, with different tasks done by
// different parts of the program: parsing the source code into an abstract
// representation, analysing the block structure, evaluating expressions,
// and then extracting the results into a form consumable by the rest of
// the program.
//
// This function does all of those steps in one call, going directly from
// source code to a populated Go struct value.
//
// The "filename" and "src" arguments describe the input configuration. The
// filename is used to add source location context to any returned error
// messages and its suffix will choose one of the two supported syntaxes:
// ".hcl" for native syntax, and ".json" for HCL JSON. The src must therefore
// contain a sequence of bytes that is valid for the selected syntax.
//
// The "ctx" argument provides variables and functions for use during
// expression evaluation. Applications that need no variables nor functions
// can just pass nil.
//
// The "target" argument must be a pointer to a value of a struct type,
// with struct tags as defined by the sibling package "gohcl".
//
// The return type is error but any non-nil error is guaranteed to be
// type-assertable to hcl.Diagnostics for applications that wish to access
// the full error details.
//
// This is a very opinionated function that is intended to serve the needs of
// applications that are just using HCL for simple configuration and don't
// need detailed control over the decoding process. Because this function is
// just wrapping functionality elsewhere, if it doesn't meet your needs then
// please consider copying it into your program and adapting it as needed.
func Decode(filename string, src []byte, ctx *hcl.EvalContext, target interface{}) error {
	var file *hcl.File
	var diags hcl.Diagnostics

	switch suffix := strings.ToLower(filepath.Ext(filename)); suffix {
	case ".hcl":
		file, diags = hclsyntax.ParseConfig(src, filename, hcl.Pos{Line: 1, Column: 1})
	case ".json":
		file, diags = json.Parse(src, filename)
	default:
		diags = diags.Append(&hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Unsupported file format",
			Detail:   fmt.Sprintf("Cannot read from %s: unrecognized file format suffix %q.", filename, suffix),
		})
		return diags
	}
	if diags.HasErrors() {
		return diags
	}

	diags = gohcl.DecodeBody(file.Body, ctx, target)
	if diags.HasErrors() {
		return diags
	}
	return nil
}

// DecodeFile is a wrapper around Decode that first reads the given filename
// from disk. See the Decode documentation for more information.
func DecodeFile(filename string, ctx *hcl.EvalContext, target interface{}) error {
	src, err := ioutil.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return hcl.Diagnostics{
				{
					Severity: hcl.DiagError,
					Summary:  "Configuration file not found",
					Detail:   fmt.Sprintf("The configuration file %s does not exist.", filename),
				},
			}
		}
		return hcl.Diagnostics{
			{
				Severity: hcl.DiagError,
				Summary:  "Failed to read configuration",
				Detail:   fmt.Sprintf("Can't read %s: %s.", filename, err),
			},
		}
	}

	return Decode(filename, src, ctx, target)
}
