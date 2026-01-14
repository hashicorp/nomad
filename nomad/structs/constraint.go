// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1
package structs

import (
	"fmt"
	"slices"
	"strings"
)

var (
	// validConstraintExactTargets are the exact targets that can be used within
	// a job specification constraint target.
	validConstraintExactTargets = []string{
		"node.unique.id",
		"node.datacenter",
		"node.unique.name",
		"node.class",
		"node.pool",
	}

	// validConstraintPrefixTargets are the valid prefixes on constraint targets
	// used within a job specification constraint.
	validConstraintPrefixTargets = []string{
		"attr.",
		"meta.",
	}
)

// validateConstraintAttribute ensures the constraint attribute is valid. It
// does this by ensuring any interpolated field can be handled by the
// resolveTarget function.
func validateConstraintAttribute(target string) error {

	// If no prefix delimieter is included, we assume this is a literal value
	// and is therefore valid.
	if !strings.HasPrefix(target, "${") {
		return nil
	}

	// Must have closing brace
	if !strings.HasSuffix(target, "}") {
		return fmt.Errorf("attribute %q is missing a closing brace", target)
	}

	// Extract the interpolatable content between the delimiters.
	interior := strings.TrimSuffix(strings.TrimPrefix(target, "${"), "}")

	// Perform our exact target matching first. If the target does not hit this
	// exact match, we will fall through to the prefix match check.
	if slices.Contains(validConstraintExactTargets, interior) {
		return nil
	}

	// Check the target against our valid prefixes.
	for _, prefix := range validConstraintPrefixTargets {
		if strings.HasPrefix(interior, prefix) {
			return nil
		}
	}

	// If we have reached this point, the target has not matched any valid
	// options. Return an error that includes the target string, so it is
	// immediately clear what constraint failed as job specifications can
	// include many constraint blocks.
	return fmt.Errorf("unsupported attribute %q", target)
}
