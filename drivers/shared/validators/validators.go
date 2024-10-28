// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package validators

import (
	"fmt"
	"os/user"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/lib/idset"
	"github.com/hashicorp/nomad/client/lib/numalib/hw"
)

type validator struct {
	// DeniedHostUids configures which host uids are disallowed
	deniedUIDs *idset.Set[hw.UserID]

	// DeniedHostGids configures which host gids are disallowed
	deniedGIDs *idset.Set[hw.GroupID]

	// logger will log to the Nomad agent
	logger hclog.Logger
}

// IDRange defines a range of uids or gids (to eventually restrict)
type IDRange struct {
	Lower uint64 `codec:"from"`
	Upper uint64 `codec:"to"`
}

func NewValidator(logger hclog.Logger, deniedHostUIDs, deniedHostGIDs string) (*validator, error) {
	// TODO: Validate set, idset assumes its valid
	dHostUID := idset.Parse[hw.UserID](deniedHostUIDs)
	dHostGID := idset.Parse[hw.GroupID](deniedHostGIDs)

	v := &validator{
		deniedUIDs: dHostUID,
		deniedGIDs: dHostGID,
		logger:     logger,
	}

	return v, nil
}

// HasValidIds is used when running a task to ensure the
// given user is in the ID range defined in the task config
func (v *validator) HasValidIDs(user *user.User) error {
	uid, err := getUserID(user)
	if err != nil {
		return fmt.Errorf("validator: %w", err)
	}

	// check uids
	if v.deniedUIDs.Contains(uid) {
		return fmt.Errorf("running as uid %d is disallowed", uid)
	}

	gids, err := getGroupID(user)
	if err != nil {
		return fmt.Errorf("validator:  %w", err)
	}

	// check gids
	for _, gid := range gids {
		if v.deniedGIDs.Contains(gid) {
			return fmt.Errorf("running as gid %d is disallowed", gid)
		}
	}

	return nil
}

/* // ParseIdRange is used to ensure that the configuration for ID ranges is valid.
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
*/
