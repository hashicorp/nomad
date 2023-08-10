// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package addrs

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
)

// Reference describes a reference to an address with source location
// information.
type Reference struct {
	Subject     Referenceable
	SourceRange hcl.Range
	Remaining   hcl.Traversal
}

// ParseRef attempts to extract a referencable address from the prefix of the
// given traversal, which must be an absolute traversal or this function
// will panic.
//
// If no error diagnostics are returned, the returned reference includes the
// address that was extracted, the source range it was extracted from, and any
// remaining relative traversal that was not consumed as part of the
// reference.
//
// If error diagnostics are returned then the Reference value is invalid and
// must not be used.
func ParseRef(traversal hcl.Traversal) (*Reference, hcl.Diagnostics) {
	ref, diags := parseRef(traversal)

	// Normalize a little to make life easier for callers.
	if ref != nil {
		if len(ref.Remaining) == 0 {
			ref.Remaining = nil
		}
	}

	return ref, diags
}

func parseRef(traversal hcl.Traversal) (*Reference, hcl.Diagnostics) {
	var diags hcl.Diagnostics

	root := traversal.RootName()
	rootRange := traversal[0].SourceRange()

	switch root {

	case "var":
		name, rng, remain, diags := parseSingleAttrRef(traversal)
		return &Reference{
			Subject:     InputVariable{Name: name},
			SourceRange: rng,
			Remaining:   remain,
		}, diags

	default:
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Unhandled reference type",
			Detail:   `Currently parseRef can only parse "var" references.`,
			Subject:  &rootRange,
		})
	}
	return nil, diags
}

func parseSingleAttrRef(traversal hcl.Traversal) (string, hcl.Range, hcl.Traversal, hcl.Diagnostics) {
	var diags hcl.Diagnostics

	root := traversal.RootName()
	rootRange := traversal[0].SourceRange()

	if len(traversal) < 2 {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid reference",
			Detail:   fmt.Sprintf("The %q object cannot be accessed directly. Instead, access one of its attributes.", root),
			Subject:  &rootRange,
		})
		return "", hcl.Range{}, nil, diags
	}
	if attrTrav, ok := traversal[1].(hcl.TraverseAttr); ok {
		return attrTrav.Name, hcl.RangeBetween(rootRange, attrTrav.SrcRange), traversal[2:], diags
	}
	diags = diags.Append(&hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  "Invalid reference",
		Detail:   fmt.Sprintf("The %q object does not support this operation.", root),
		Subject:  traversal[1].SourceRange().Ptr(),
	})
	return "", hcl.Range{}, nil, diags
}
