// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package validators

import (
	"fmt"
	"strings"
)

// IDRange defines a range of uids or gids (to eventually restrict)
type IDRange struct {
	Lower uint64 `codec:"from"`
	Upper uint64 `codec:"to"`
}

// ParseIdRange is used to ensure that the configuration for ID ranges is valid.
func ParseIdRange(rangeType string, deniedRanges string) ([]IDRange, error) {
	var idRanges []IDRange
	parts := strings.Split(deniedRanges, ",")

	// exit early if empty string
	if len(parts) == 1 && parts[0] == "" {
		return idRanges, nil
	}

	for _, rangeStr := range parts {
		idRange, err := parseRangeString(rangeStr)
		if err != nil {
			return nil, fmt.Errorf("invalid %s: %w", rangeType, err)
		}

		idRanges = append(idRanges, *idRange)
	}

	return idRanges, nil
}
