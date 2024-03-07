// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !windows

package validators

import (
	"fmt"
	"os/user"
	"strconv"
	"strings"

	"github.com/hashicorp/nomad/nomad/structs"
)

// ParseIdRange is used to ensure that the configuration for ID ranges is valid.
func ParseIdRange(rangeType string, deniedRanges string) ([]structs.IDRange, error) {
	var idRanges []structs.IDRange
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

type userLookupFn func(string) (*user.User, error)

// HasValidIds is used when running a task to ensure the
// given user is in the ID range defined in the task config
func HasValidIds(userLookupFn userLookupFn, usernameToLookup string, deniedHostUIDs, deniedHostGIDs []structs.IDRange) error {

	// look up user on host given username

	u, err := userLookupFn(usernameToLookup)
	if err != nil {
		return fmt.Errorf("failed to identify user %q: %w", usernameToLookup, err)
	}
	uid, err := strconv.ParseUint(u.Uid, 10, 32)
	if err != nil {
		return fmt.Errorf("unable to convert userid %s to integer", u.Uid)
	}

	// check uids

	for _, uidRange := range deniedHostUIDs {
		if uid >= uidRange.Lower && uid <= uidRange.Upper {
			return fmt.Errorf("running as uid %d is disallowed", uid)
		}
	}

	// check gids

	gidStrings, err := u.GroupIds()
	if err != nil {
		return fmt.Errorf("unable to lookup user's group membership: %w", err)
	}
	gids := make([]uint64, len(gidStrings))

	for _, gidString := range gidStrings {
		u, err := strconv.ParseUint(gidString, 10, 32)
		if err != nil {
			return fmt.Errorf("unable to convert user's group %q to integer: %w", gidString, err)
		}

		gids = append(gids, u)
	}

	for _, gidRange := range deniedHostGIDs {
		for _, gid := range gids {
			if gid >= gidRange.Lower && gid <= gidRange.Upper {
				return fmt.Errorf("running as gid %d is disallowed", gid)
			}
		}
	}

	return nil
}

func parseRangeString(boundsString string) (*structs.IDRange, error) {
	uidDenyRangeParts := strings.Split(boundsString, "-")

	var idRange structs.IDRange

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
