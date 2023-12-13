// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package mock

import (
	"time"
)

// parseDuration parses a duration string, like time.ParseDuration
// but is empty string friendly, returns a zero time duration
func parseDuration(s string) (time.Duration, error) {
	if s == "" {
		return time.Duration(0), nil
	}

	// try to parse it as duration
	return time.ParseDuration(s)
}
