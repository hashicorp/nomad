// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package validators

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/lib/idset"
	"github.com/hashicorp/nomad/helper/users"
)

var (
	ErrInvalidBound = errors.New("range bound not valid")
	//ErrEmptyRange   = errors.New("range value cannot be empty")
	ErrInvalidRange = errors.New("lower bound cannot be greater than upper bound")
)

type (

	// A GroupID (GID) represents a unique numerical value assigned to each user group.
	GroupID uint64

	// A UserID represents a unique numerical value assigned to each user account.
	UserID uint64
)

type Validator struct {
	// DeniedHostUids configures which host uids are disallowed
	deniedUIDs *idset.Set[UserID]

	// DeniedHostGids configures which host gids are disallowed
	deniedGIDs *idset.Set[GroupID]

	// logger will log to the Nomad agent
	logger hclog.Logger
}

func NewValidator(logger hclog.Logger, deniedHostUIDs, deniedHostGIDs string) (*Validator, error) {
	valLogger := logger.Named("id_validator")

	err := validateIDRange("deniedHostUIDs", deniedHostUIDs)
	if err != nil {
		return nil, err
	}
	valLogger.Debug("user range configured", "denied range", deniedHostUIDs)

	err = validateIDRange("deniedHostGIDs", deniedHostGIDs)
	if err != nil {
		return nil, err
	}
	valLogger.Debug("group range configured", "denied range", deniedHostGIDs)

	v := &Validator{
		deniedUIDs: idset.Parse[UserID](deniedHostUIDs),
		deniedGIDs: idset.Parse[GroupID](deniedHostGIDs),
		logger:     valLogger,
	}

	return v, nil
}

// HasValidIDs is used when running a task to ensure the
// given user is in the ID range defined in the task config
func (v *Validator) HasValidIDs(userName string) error {
	user, err := users.Lookup(userName)
	if err != nil {
		return fmt.Errorf("failed to identify user %q: %w", userName, err)
	}

	uid, err := getUserID(user)
	if err != nil {
		return fmt.Errorf("validator: %w", err)
	}

	// check uids
	if v.deniedUIDs.Contains(uid) {
		return fmt.Errorf("running as uid %d is disallowed", uid)
	}

	gids, err := getGroupsID(user)
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

// validateIDRange is used to ensure that the configuration for ID ranges is valid
// by checking the syntax and bounds.
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
