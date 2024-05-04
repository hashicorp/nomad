// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package validators

import (
	"fmt"
	"strconv"
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

func parseRangeString(boundsString string) (*IDRange, error) {
	uidDenyRangeParts := strings.Split(boundsString, "-")

	var idRange IDRange

	switch len(uidDenyRangeParts) {
	case 0:
		return nil, fmt.Errorf("range value cannot be empty")
	case 1:
		disallowedIdStr := uidDenyRangeParts[0]
		disallowedIdInt, err := strconv.ParseUint(disallowedIdStr, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("range bound not valid, invalid bound: %q ", disallowedIdInt)
		}

		idRange.Lower = disallowedIdInt
		idRange.Upper = disallowedIdInt
	case 2:
		lowerBoundStr := uidDenyRangeParts[0]
		upperBoundStr := uidDenyRangeParts[1]

		lowerBoundInt, err := strconv.ParseUint(lowerBoundStr, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid bound: %q", lowerBoundStr)
		}

		upperBoundInt, err := strconv.ParseUint(upperBoundStr, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid bound: %q", upperBoundStr)
		}

		if lowerBoundInt > upperBoundInt {
			return nil, fmt.Errorf("invalid range %q, lower bound cannot be greater than upper bound", boundsString)
		}

		idRange.Lower = lowerBoundInt
		idRange.Upper = upperBoundInt
	}

	return &idRange, nil
}
