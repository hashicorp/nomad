// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package helper

import (
	"fmt"
	"strings"

	"github.com/hashicorp/go-multierror"
)

// MergeMultierrorWarnings takes warnings and merges them into a returnable
// string. This method is used to return API and CLI errors.
func MergeMultierrorWarnings(errs ...error) string {
	if len(errs) == 0 {
		return ""
	}

	var mErr multierror.Error
	_ = multierror.Append(&mErr, errs...)
	mErr.ErrorFormat = warningsFormatter

	return mErr.Error()
}

// warningsFormatter is used to format warnings.
func warningsFormatter(es []error) string {
	sb := strings.Builder{}

	switch len(es) {
	case 0:
		return ""
	case 1:
		sb.WriteString("1 warning:\n")
	default:
		sb.WriteString(fmt.Sprintf("%d warnings:\n", len(es)))
	}

	for _, err := range es {
		sb.WriteString(fmt.Sprintf("\n* %s", strings.TrimSpace(err.Error())))
	}

	return sb.String()
}
