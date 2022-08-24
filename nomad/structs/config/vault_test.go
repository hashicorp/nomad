package config

import (
	"reflect"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper"
	"github.com/stretchr/testify/require"
)

func TestVaultConfig_Merge(t *testing.T) {
	ci.Parallel(t)

	c1 := &VaultConfig{
		Enabled:              helper.BoolToPtr(false),
		Token:                "1",
		Role:                 "1",
		AllowUnauthenticated: helper.BoolToPtr(true),
		TaskTokenTTL:         "1",
		Addr:                 "1",
		TLSCaFile:            "1",
		TLSCaPath:            "1",
		TLSCertFile:          "1",
		TLSKeyFile:           "1",
		TLSSkipVerify:        helper.BoolToPtr(true),
		TLSServerName:        "1",
	}

	c2 := &VaultConfig{
		Enabled:              helper.BoolToPtr(true),
		Token:                "2",
		Role:                 "2",
		AllowUnauthenticated: helper.BoolToPtr(false),
		TaskTokenTTL:         "2",
		Addr:                 "2",
		TLSCaFile:            "2",
		TLSCaPath:            "2",
		TLSCertFile:          "2",
		TLSKeyFile:           "2",
		TLSSkipVerify:        nil,
		TLSServerName:        "2",
	}

	e := &VaultConfig{
		Enabled:              helper.BoolToPtr(true),
		Token:                "2",
		Role:                 "2",
		AllowUnauthenticated: helper.BoolToPtr(false),
		TaskTokenTTL:         "2",
		Addr:                 "2",
		TLSCaFile:            "2",
		TLSCaPath:            "2",
		TLSCertFile:          "2",
		TLSKeyFile:           "2",
		TLSSkipVerify:        helper.BoolToPtr(true),
		TLSServerName:        "2",
	}

	result := c1.Merge(c2)
	if !reflect.DeepEqual(result, e) {
		t.Fatalf("bad:\n%#v\n%#v", result, e)
	}
}

func TestVaultConfig_Equals(t *testing.T) {
	ci.Parallel(t)

	c1 := &VaultConfig{
		Enabled:              helper.BoolToPtr(false),
		Token:                "1",
		Role:                 "1",
		Namespace:            "1",
		AllowUnauthenticated: helper.BoolToPtr(true),
		TaskTokenTTL:         "1",
		Addr:                 "1",
		ConnectionRetryIntv:  time.Second,
		TLSCaFile:            "1",
		TLSCaPath:            "1",
		TLSCertFile:          "1",
		TLSKeyFile:           "1",
		TLSSkipVerify:        helper.BoolToPtr(true),
		TLSServerName:        "1",
	}

	c2 := &VaultConfig{
		Enabled:              helper.BoolToPtr(false),
		Token:                "1",
		Role:                 "1",
		Namespace:            "1",
		AllowUnauthenticated: helper.BoolToPtr(true),
		TaskTokenTTL:         "1",
		Addr:                 "1",
		ConnectionRetryIntv:  time.Second,
		TLSCaFile:            "1",
		TLSCaPath:            "1",
		TLSCertFile:          "1",
		TLSKeyFile:           "1",
		TLSSkipVerify:        helper.BoolToPtr(true),
		TLSServerName:        "1",
	}

	require.True(t, c1.Equals(c2))

	c3 := &VaultConfig{
		Enabled:              helper.BoolToPtr(true),
		Token:                "1",
		Role:                 "1",
		Namespace:            "1",
		AllowUnauthenticated: helper.BoolToPtr(true),
		TaskTokenTTL:         "1",
		Addr:                 "1",
		ConnectionRetryIntv:  time.Second,
		TLSCaFile:            "1",
		TLSCaPath:            "1",
		TLSCertFile:          "1",
		TLSKeyFile:           "1",
		TLSSkipVerify:        helper.BoolToPtr(true),
		TLSServerName:        "1",
	}

	c4 := &VaultConfig{
		Enabled:              helper.BoolToPtr(false),
		Token:                "1",
		Role:                 "1",
		Namespace:            "1",
		AllowUnauthenticated: helper.BoolToPtr(true),
		TaskTokenTTL:         "1",
		Addr:                 "1",
		ConnectionRetryIntv:  time.Second,
		TLSCaFile:            "1",
		TLSCaPath:            "1",
		TLSCertFile:          "1",
		TLSKeyFile:           "1",
		TLSSkipVerify:        helper.BoolToPtr(true),
		TLSServerName:        "1",
	}

	require.False(t, c3.Equals(c4))
}
