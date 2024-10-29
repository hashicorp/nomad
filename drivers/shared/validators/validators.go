// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package validators

import (
	"errors"
	"fmt"
	"os/user"
	"strconv"
	"strings"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/lib/idset"
	"github.com/hashicorp/nomad/client/lib/numalib/hw"
)

var (
	ErrInvalidBound = errors.New("range bound not valid")
	ErrEmptyRange   = errors.New("range value cannot be empty")
	ErrInvalidRange = errors.New("lower bound cannot be greater than upper bound")
)

type validator struct {
	// DeniedHostUids configures which host uids are disallowed
	deniedUIDs *idset.Set[hw.UserID]

	// DeniedHostGids configures which host gids are disallowed
	deniedGIDs *idset.Set[hw.GroupID]

	// logger will log to the Nomad agent
	logger hclog.Logger
}

func NewValidator(logger hclog.Logger, deniedHostUIDs, deniedHostGIDs string) (*validator, error) {
	err := validateIDRange("deniedHostUIDs", deniedHostUIDs)
	if err != nil {
		return nil, err
	}

	err = validateIDRange("deniedHostGIDs", deniedHostGIDs)
	if err != nil {
		return nil, err
	}

	v := &validator{
		deniedUIDs: idset.Parse[hw.UserID](deniedHostUIDs),
		deniedGIDs: idset.Parse[hw.GroupID](deniedHostGIDs),
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

// ParseIdRange is used to ensure that the configuration for ID ranges is valid.
func validateIDRange(rangeType string, deniedRanges string) error {

	parts := strings.Split(deniedRanges, ",")

	// exit early if empty string
	if len(parts) == 1 && parts[0] == "" {
		return nil
	}

	for _, rangeStr := range parts {
		err := validateBounds(rangeStr)
		if err != nil {
			return fmt.Errorf("invalid range %s \"%s\": %w", rangeType, rangeStr, err)
		}
	}

	return nil
}

func validateBounds(boundsString string) error {
	uidDenyRangeParts := strings.Split(boundsString, "-")

	switch len(uidDenyRangeParts) {
	case 0:
		return ErrEmptyRange

	case 1:
		disallowedIdStr := uidDenyRangeParts[0]
		if _, err := strconv.ParseUint(disallowedIdStr, 10, 32); err != nil {
			return ErrInvalidBound
		}

	case 2:
		lowerBoundStr := uidDenyRangeParts[0]
		upperBoundStr := uidDenyRangeParts[1]

		lowerBoundInt, err := strconv.ParseUint(lowerBoundStr, 10, 32)
		if err != nil {
			return ErrInvalidBound
		}

		upperBoundInt, err := strconv.ParseUint(upperBoundStr, 10, 32)
		if err != nil {
			return ErrInvalidBound
		}

		if lowerBoundInt > upperBoundInt {
			return ErrInvalidRange
		}
	}

	return nil
}
