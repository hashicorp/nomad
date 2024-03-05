// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package validators

import (
	"fmt"
	"os/user"
	"strconv"
	"strings"
)

// IDRange defines a range of uids or gids (to eventually restrict)
type IDRange struct {
	Lower uint64 `codec:"from"`
	Upper uint64 `codec:"to"`
}

// IDRangeValid is used to ensure that the configuration for ID ranges is valid.
func IDRangeValid(rangeType string, deniedRanges string) error {
	_, err := parseRanges(deniedRanges)
	if err != nil {
		return fmt.Errorf("invalid %s value %q: %v", rangeType, deniedRanges, err)
	}

	return nil
}

type userLookupFn func(string) (*user.User, error)

// UserInRange is used when running a task to ensure the
// given user is in the ID range defined in the task config
func UserInRange(userLookupFn userLookupFn, usernameToLookup string, deniedHostUIDs, deniedHostGIDs string) error {

	// look up user on host given username

	u, err := userLookupFn(usernameToLookup)
	if err != nil {
		return fmt.Errorf("failed to identify user %q: %v", usernameToLookup, err)
	}
	uid, err := strconv.ParseUint(u.Uid, 10, 32)
	if err != nil {
		return fmt.Errorf("unable to convert userid %s to integer", u.Uid)
	}

	// check uids

	uidRanges, err := parseRanges(deniedHostUIDs)
	if err != nil {
		return fmt.Errorf("invalid denied_host_uids value %q: %v", deniedHostUIDs, err)
	}

	for _, uidRange := range uidRanges {
		if uid >= uidRange.Lower && uid <= uidRange.Upper {
			return fmt.Errorf("running as uid %d is disallowed", uid)
		}
	}

	// check gids

	gidStrings, err := u.GroupIds()
	if err != nil {
		return fmt.Errorf("unable to lookup user's group membership: %v", err)
	}
	gids := make([]uint64, len(gidStrings))

	for _, gidString := range gidStrings {
		u, err := strconv.ParseUint(gidString, 10, 32)
		if err != nil {
			return fmt.Errorf("unable to convert user's group %q to integer", gidString)
		}

		gids = append(gids, u)
	}

	gidRanges, err := parseRanges(deniedHostGIDs)
	if err != nil {
		return fmt.Errorf("invalid denied_host_gids value %q: %v", deniedHostGIDs, err)
	}

	for _, gidRange := range gidRanges {
		for _, gid := range gids {
			if gid >= gidRange.Lower && gid <= gidRange.Upper {
				return fmt.Errorf("running as gid %d is disallowed", gid)
			}
		}
	}

	return nil
}

func parseRanges(ranges string) ([]IDRange, error) {
	var idRanges []IDRange

	parts := strings.Split(ranges, ",")

	// exit early if empty string
	if len(parts) == 1 && parts[0] == "" {
		return idRanges, nil
	}

	for _, rangeStr := range parts {
		idRange, err := parseRangeString(rangeStr)
		if err != nil {
			return nil, err
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
		return nil, fmt.Errorf("range cannot be empty, invalid range: \"%q\" ", boundsString)
	case 1:
		singleBound := uidDenyRangeParts[0]
		singleBoundInt, err := strconv.ParseUint(singleBound, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("range bound not valid, invalid bound: \"%q\" ", singleBoundInt)
		}

		idRange.Lower = singleBoundInt
		idRange.Upper = singleBoundInt
	case 2:
		boundAStr := uidDenyRangeParts[0]
		boundBStr := uidDenyRangeParts[1]

		boundAInt, err := strconv.ParseUint(boundAStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid range %q, invalid bound: \"%q\" ", boundsString, boundAStr)
		}

		boundBInt, err := strconv.ParseUint(boundBStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid range %q, invalid bound: \"%q\" ", boundsString, boundBStr)
		}

		if boundAInt > boundBInt {
			return nil, fmt.Errorf("invalid range %q, lower bound cannot be greater than upper bound", boundsString)
		}

		idRange.Lower = boundAInt
		idRange.Upper = boundBInt
	}

	return &idRange, nil
}
