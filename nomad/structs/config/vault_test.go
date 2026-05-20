// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package config

import (
	"reflect"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
)

func TestVaultConfig_Merge(t *testing.T) {
	ci.Parallel(t)

	c1 := &VaultConfig{
		Enabled:            new(false),
		Role:               "1",
		Addr:               "1",
		JWTAuthBackendPath: "jwt",
		TLSCaFile:          "1",
		TLSCaPath:          "1",
		TLSCertFile:        "1",
		TLSKeyFile:         "1",
		TLSSkipVerify:      new(true),
		TLSServerName:      "1",
		DefaultIdentity:    nil,
	}

	c2 := &VaultConfig{
		Enabled:            new(true),
		Role:               "2",
		Addr:               "2",
		JWTAuthBackendPath: "jwt2",
		TLSCaFile:          "2",
		TLSCaPath:          "2",
		TLSCertFile:        "2",
		TLSKeyFile:         "2",
		TLSSkipVerify:      nil,
		TLSServerName:      "2",
		DefaultIdentity: &WorkloadIdentityConfig{
			Audience: []string{"vault.dev"},
			Env:      new(true),
			File:     new(false),
		},
	}

	e := &VaultConfig{
		Enabled:            new(true),
		Role:               "2",
		Addr:               "2",
		JWTAuthBackendPath: "jwt2",
		TLSCaFile:          "2",
		TLSCaPath:          "2",
		TLSCertFile:        "2",
		TLSKeyFile:         "2",
		TLSSkipVerify:      new(true),
		TLSServerName:      "2",
		DefaultIdentity: &WorkloadIdentityConfig{
			Audience: []string{"vault.dev"},
			Env:      new(true),
			File:     new(false),
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
		Enabled:             new(false),
		Role:                "1",
		Namespace:           "1",
		Addr:                "1",
		JWTAuthBackendPath:  "jwt",
		ConnectionRetryIntv: time.Second,
		TLSCaFile:           "1",
		TLSCaPath:           "1",
		TLSCertFile:         "1",
		TLSKeyFile:          "1",
		TLSSkipVerify:       new(true),
		TLSServerName:       "1",
		DefaultIdentity: &WorkloadIdentityConfig{
			Audience: []string{"vault.dev"},
			Env:      new(true),
			File:     new(false),
		},
	}

	c2 := &VaultConfig{
		Enabled:             new(false),
		Role:                "1",
		Namespace:           "1",
		Addr:                "1",
		JWTAuthBackendPath:  "jwt",
		ConnectionRetryIntv: time.Second,
		TLSCaFile:           "1",
		TLSCaPath:           "1",
		TLSCertFile:         "1",
		TLSKeyFile:          "1",
		TLSSkipVerify:       new(true),
		TLSServerName:       "1",
		DefaultIdentity: &WorkloadIdentityConfig{
			Audience: []string{"vault.dev"},
			Env:      new(true),
			File:     new(false),
		},
	}

	must.Equal(t, c1, c2)

	c3 := &VaultConfig{
		Enabled:             new(true),
		Role:                "1",
		Namespace:           "1",
		Addr:                "1",
		ConnectionRetryIntv: time.Second,
		TLSCaFile:           "1",
		TLSCaPath:           "1",
		TLSCertFile:         "1",
		TLSKeyFile:          "1",
		TLSSkipVerify:       new(true),
		TLSServerName:       "1",
		DefaultIdentity: &WorkloadIdentityConfig{
			Audience: []string{"vault.dev"},
			Env:      new(true),
			File:     new(false),
		},
	}

	c4 := &VaultConfig{
		Enabled:             new(false),
		Role:                "1",
		Namespace:           "1",
		Addr:                "1",
		ConnectionRetryIntv: time.Second,
		TLSCaFile:           "1",
		TLSCaPath:           "1",
		TLSCertFile:         "1",
		TLSKeyFile:          "1",
		TLSSkipVerify:       new(true),
		TLSServerName:       "1",
		DefaultIdentity: &WorkloadIdentityConfig{
			Audience: []string{"vault.io"},
			Env:      new(false),
			File:     new(true),
		},
	}

	must.NotEqual(t, c3, c4)
}
