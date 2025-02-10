// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package fingerprint

import (
	"errors"
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/shoenig/go-landlock"
	"github.com/shoenig/test/must"
)

func TestLandlockFingerprint(t *testing.T) {
	testutil.RequireLinux(t)
	ci.Parallel(t)

	version, err := landlock.Detect()
	must.NoError(t, err)

	logger := testlog.HCLogger(t)
	f := NewLandlockFingerprint(logger)

	var response FingerprintResponse
	err = f.Fingerprint(nil, &response)
	must.NoError(t, err)

	result := response.Attributes[landlockKey]
	switch version {
	case 0:
		must.Eq(t, "", result)
	default:
		must.Eq(t, fmt.Sprintf("v%d", version), result)
	}
}

func TestLandlockFingerprint_absent(t *testing.T) {
	ci.Parallel(t)

	logger := testlog.HCLogger(t)
	f := NewLandlockFingerprint(logger)
	f.(*LandlockFingerprint).detector = func() (int, error) {
		return 0, nil
	}

	var response FingerprintResponse
	err := f.Fingerprint(nil, &response)
	must.NoError(t, err)

	_, exists := response.Attributes[landlockKey]
	must.False(t, exists)
}

func TestLandlockFingerprint_error(t *testing.T) {
	ci.Parallel(t)

	logger := testlog.HCLogger(t)
	f := NewLandlockFingerprint(logger)
	f.(*LandlockFingerprint).detector = func() (int, error) {
		return 0, errors.New("oops")
	}

	var response FingerprintResponse
	err := f.Fingerprint(nil, &response)
	must.NoError(t, err)

	_, exists := response.Attributes[landlockKey]
	must.False(t, exists)
}
