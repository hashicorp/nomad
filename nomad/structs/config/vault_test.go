// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package config

import (
	"reflect"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/shoenig/test/must"
)

func TestVaultConfig_Merge(t *testing.T) {
	ci.Parallel(t)

	c1 := &VaultConfig{
		Enabled:              pointer.Of(false),
		Token:                "1",
		Role:                 "1",
		AllowUnauthenticated: pointer.Of(true),
		TaskTokenTTL:         "1",
		Addr:                 "1",
		TLSCaFile:            "1",
		TLSCaPath:            "1",
		TLSCertFile:          "1",
		TLSKeyFile:           "1",
		TLSSkipVerify:        pointer.Of(true),
		TLSServerName:        "1",
		UseIdentity:          pointer.Of(false),
		DefaultIdentity:      nil,
	}

	c2 := &VaultConfig{
		Enabled:              pointer.Of(true),
		Token:                "2",
		Role:                 "2",
		AllowUnauthenticated: pointer.Of(false),
		TaskTokenTTL:         "2",
		Addr:                 "2",
		TLSCaFile:            "2",
		TLSCaPath:            "2",
		TLSCertFile:          "2",
		TLSKeyFile:           "2",
		TLSSkipVerify:        nil,
		TLSServerName:        "2",
		UseIdentity:          pointer.Of(true),
		DefaultIdentity: &WorkloadIdentityConfig{
			Audience: []string{"vault.dev"},
			Env:      pointer.Of(true),
			File:     pointer.Of(false),
		},
	}

	e := &VaultConfig{
		Enabled:              pointer.Of(true),
		Token:                "2",
		Role:                 "2",
		AllowUnauthenticated: pointer.Of(false),
		TaskTokenTTL:         "2",
		Addr:                 "2",
		TLSCaFile:            "2",
		TLSCaPath:            "2",
		TLSCertFile:          "2",
		TLSKeyFile:           "2",
		TLSSkipVerify:        pointer.Of(true),
		TLSServerName:        "2",
		UseIdentity:          pointer.Of(true),
		DefaultIdentity: &WorkloadIdentityConfig{
			Audience: []string{"vault.dev"},
			Env:      pointer.Of(true),
			File:     pointer.Of(false),
		},
	}

	result := c1.Merge(c2)
	if !reflect.DeepEqual(result, e) {
		t.Fatalf("bad:\n%#v\n%#v", result, e)
	}
}

func TestVaultConfig_Equals(t *testing.T) {
	ci.Parallel(t)

	c1 := &VaultConfig{
		Enabled:              pointer.Of(false),
		Token:                "1",
		Role:                 "1",
		Namespace:            "1",
		AllowUnauthenticated: pointer.Of(true),
		TaskTokenTTL:         "1",
		Addr:                 "1",
		ConnectionRetryIntv:  time.Second,
		TLSCaFile:            "1",
		TLSCaPath:            "1",
		TLSCertFile:          "1",
		TLSKeyFile:           "1",
		TLSSkipVerify:        pointer.Of(true),
		TLSServerName:        "1",
		UseIdentity:          pointer.Of(true),
		DefaultIdentity: &WorkloadIdentityConfig{
			Audience: []string{"vault.dev"},
			Env:      pointer.Of(true),
			File:     pointer.Of(false),
		},
	}

	c2 := &VaultConfig{
		Enabled:              pointer.Of(false),
		Token:                "1",
		Role:                 "1",
		Namespace:            "1",
		AllowUnauthenticated: pointer.Of(true),
		TaskTokenTTL:         "1",
		Addr:                 "1",
		ConnectionRetryIntv:  time.Second,
		TLSCaFile:            "1",
		TLSCaPath:            "1",
		TLSCertFile:          "1",
		TLSKeyFile:           "1",
		TLSSkipVerify:        pointer.Of(true),
		TLSServerName:        "1",
		UseIdentity:          pointer.Of(true),
		DefaultIdentity: &WorkloadIdentityConfig{
			Audience: []string{"vault.dev"},
			Env:      pointer.Of(true),
			File:     pointer.Of(false),
		},
	}

	must.Equal(t, c1, c2)

	c3 := &VaultConfig{
		Enabled:              pointer.Of(true),
		Token:                "1",
		Role:                 "1",
		Namespace:            "1",
		AllowUnauthenticated: pointer.Of(true),
		TaskTokenTTL:         "1",
		Addr:                 "1",
		ConnectionRetryIntv:  time.Second,
		TLSCaFile:            "1",
		TLSCaPath:            "1",
		TLSCertFile:          "1",
		TLSKeyFile:           "1",
		TLSSkipVerify:        pointer.Of(true),
		TLSServerName:        "1",
		UseIdentity:          pointer.Of(true),
		DefaultIdentity: &WorkloadIdentityConfig{
			Audience: []string{"vault.dev"},
			Env:      pointer.Of(true),
			File:     pointer.Of(false),
		},
	}

	c4 := &VaultConfig{
		Enabled:              pointer.Of(false),
		Token:                "1",
		Role:                 "1",
		Namespace:            "1",
		AllowUnauthenticated: pointer.Of(true),
		TaskTokenTTL:         "1",
		Addr:                 "1",
		ConnectionRetryIntv:  time.Second,
		TLSCaFile:            "1",
		TLSCaPath:            "1",
		TLSCertFile:          "1",
		TLSKeyFile:           "1",
		TLSSkipVerify:        pointer.Of(true),
		TLSServerName:        "1",
		UseIdentity:          pointer.Of(false),
		DefaultIdentity: &WorkloadIdentityConfig{
			Audience: []string{"vault.io"},
			Env:      pointer.Of(false),
			File:     pointer.Of(true),
		},
	}

	must.NotEqual(t, c3, c4)
}
