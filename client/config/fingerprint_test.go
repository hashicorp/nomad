// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package config

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
)

func TestFingerprint_Copy(t *testing.T) {
	ci.Parallel(t)

	t.Run("nil", func(t *testing.T) {
		var f *Fingerprint
		result := f.Copy()
		must.Nil(t, result)
	})

	t.Run("full", func(t *testing.T) {
		exitOnFailure := true
		original := &Fingerprint{
			Name:             "env_aws",
			RetryInterval:    5 * time.Minute,
			RetryIntervalHCL: "5m",
			RetryAttempts:    3,
			ExitOnFailure:    &exitOnFailure,
		}

		copied := original.Copy()

		must.Eq(t, original.Name, copied.Name)
		must.Eq(t, original.RetryInterval, copied.RetryInterval)
		must.Eq(t, original.RetryIntervalHCL, copied.RetryIntervalHCL)
		must.Eq(t, original.RetryAttempts, copied.RetryAttempts)
		must.Eq(t, *original.ExitOnFailure, *copied.ExitOnFailure)
	})

	t.Run("empty", func(t *testing.T) {
		original := &Fingerprint{}
		copied := original.Copy()

		must.NotNil(t, copied)
		must.Eq(t, "", copied.Name)
		must.Eq(t, time.Duration(0), copied.RetryInterval)
		must.Eq(t, "", copied.RetryIntervalHCL)
		must.Eq(t, 0, copied.RetryAttempts)
		must.Nil(t, copied.ExitOnFailure)
	})
}

func TestFingerprint_Merge(t *testing.T) {
	ci.Parallel(t)

	t.Run("nil receiver", func(t *testing.T) {
		var f *Fingerprint
		other := &Fingerprint{
			Name:          "env_aws",
			RetryInterval: 5 * time.Minute,
		}

		result := f.Merge(other)
		must.Eq(t, other, result)
	})

	t.Run("nil argument", func(t *testing.T) {
		exitOnFailure := true
		f := &Fingerprint{
			Name:          "env_aws",
			RetryInterval: 5 * time.Minute,
			RetryAttempts: 3,
			ExitOnFailure: &exitOnFailure,
		}

		result := f.Merge(nil)
		must.Eq(t, f.Name, result.Name)
		must.Eq(t, f.RetryInterval, result.RetryInterval)
		must.Eq(t, f.RetryAttempts, result.RetryAttempts)
		must.Eq(t, *f.ExitOnFailure, *result.ExitOnFailure)
	})

	t.Run("merge overwrites non-zero values", func(t *testing.T) {
		exitOnFailure1 := false
		exitOnFailure2 := true

		base := &Fingerprint{
			Name:             "env_aws",
			RetryInterval:    5 * time.Minute,
			RetryIntervalHCL: "5m",
			RetryAttempts:    3,
			ExitOnFailure:    &exitOnFailure1,
		}

		override := &Fingerprint{
			Name:             "env_aws",
			RetryInterval:    10 * time.Minute,
			RetryIntervalHCL: "10m",
			RetryAttempts:    5,
			ExitOnFailure:    &exitOnFailure2,
		}

		result := base.Merge(override)

		must.Eq(t, "env_aws", result.Name)
		must.Eq(t, 10*time.Minute, result.RetryInterval)
		must.Eq(t, "10m", result.RetryIntervalHCL)
		must.Eq(t, 5, result.RetryAttempts)
		must.True(t, *result.ExitOnFailure)
	})

	t.Run("merge preserves base values for zero values in override", func(t *testing.T) {
		exitOnFailure := true
		base := &Fingerprint{
			Name:             "env_aws",
			RetryInterval:    5 * time.Minute,
			RetryIntervalHCL: "5m",
			RetryAttempts:    3,
			ExitOnFailure:    &exitOnFailure,
		}

		override := &Fingerprint{
			Name: "env_aws",
		}

		result := base.Merge(override)

		must.Eq(t, "env_aws", result.Name)
		must.Eq(t, 5*time.Minute, result.RetryInterval)
		must.Eq(t, "5m", result.RetryIntervalHCL)
		must.Eq(t, 3, result.RetryAttempts)
		must.True(t, *result.ExitOnFailure)
	})

	t.Run("merge partial override", func(t *testing.T) {
		base := &Fingerprint{
			Name:          "env_azure",
			RetryInterval: 5 * time.Minute,
			RetryAttempts: 3,
		}

		newExitOnFailure := true
		override := &Fingerprint{
			Name:          "env_azure",
			RetryAttempts: 10,
			ExitOnFailure: &newExitOnFailure,
		}

		result := base.Merge(override)

		must.Eq(t, "env_azure", result.Name)
		must.Eq(t, 5*time.Minute, result.RetryInterval)
		must.Eq(t, 10, result.RetryAttempts)
		must.True(t, *result.ExitOnFailure)
	})

	t.Run("merge does not mutate original", func(t *testing.T) {
		base := &Fingerprint{
			Name:          "env_gce",
			RetryInterval: 5 * time.Minute,
			RetryAttempts: 3,
		}

		override := &Fingerprint{
			Name:          "env_gce",
			RetryInterval: 10 * time.Minute,
		}

		result := base.Merge(override)

		must.Eq(t, 5*time.Minute, base.RetryInterval)
		must.Eq(t, 3, base.RetryAttempts)
		must.Eq(t, 10*time.Minute, result.RetryInterval)
		must.Eq(t, 3, result.RetryAttempts)
	})
}

func TestFingerprint_Validate(t *testing.T) {
	ci.Parallel(t)

	t.Run("nil fingerprint", func(t *testing.T) {
		var f *Fingerprint
		must.NoError(t, f.Validate())
	})

	t.Run("empty name", func(t *testing.T) {
		f := &Fingerprint{
			Name: "",
		}
		must.ErrorContains(t, f.Validate(), "fingerprint name cannot be empty")
	})

	t.Run("invalid fingerprinter name", func(t *testing.T) {
		f := &Fingerprint{
			Name: "invalid_fingerprinter",
		}
		must.ErrorContains(t, f.Validate(), "does not support configuration")
	})

	t.Run("negative retry interval", func(t *testing.T) {
		f := &Fingerprint{
			Name:          "env_aws",
			RetryInterval: -5 * time.Minute,
		}
		must.ErrorContains(t, f.Validate(), "retry interval cannot be negative")
	})

	t.Run("retry attempts less than -1", func(t *testing.T) {
		f := &Fingerprint{
			Name:          "env_aws",
			RetryAttempts: -2,
		}
		must.ErrorContains(t, f.Validate(), "retry attempts cannot be less than -1")
	})

	t.Run("retry attempts of -1 is valid", func(t *testing.T) {
		f := &Fingerprint{
			Name:          "env_aws",
			RetryAttempts: -1,
		}
		must.NoError(t, f.Validate())
	})

	t.Run("valid env_aws fingerprint", func(t *testing.T) {
		exitOnFailure := true
		f := &Fingerprint{
			Name:          "env_aws",
			RetryInterval: 5 * time.Minute,
			RetryAttempts: 3,
			ExitOnFailure: &exitOnFailure,
		}
		must.NoError(t, f.Validate())
	})

	t.Run("valid env_azure fingerprint", func(t *testing.T) {
		f := &Fingerprint{
			Name:          "env_azure",
			RetryInterval: 10 * time.Minute,
			RetryAttempts: 5,
		}
		must.NoError(t, f.Validate())
	})

	t.Run("valid env_gce fingerprint", func(t *testing.T) {
		f := &Fingerprint{
			Name:          "env_gce",
			RetryInterval: 2 * time.Minute,
			RetryAttempts: 10,
		}
		must.NoError(t, f.Validate())
	})

	t.Run("valid env_digitalocean fingerprint", func(t *testing.T) {
		f := &Fingerprint{
			Name:          "env_digitalocean",
			RetryInterval: 1 * time.Minute,
			RetryAttempts: 0,
		}
		must.NoError(t, f.Validate())
	})

	t.Run("valid fingerprint with zero values", func(t *testing.T) {
		f := &Fingerprint{
			Name:          "env_aws",
			RetryInterval: 0,
			RetryAttempts: 0,
		}
		must.NoError(t, f.Validate())
	})

	t.Run("unknown keys", func(t *testing.T) {
		f := &Fingerprint{
			Name:          "env_aws",
			RetryInterval: 0,
			RetryAttempts: 0,
			ExtraKeysHCL:  []string{"bad"},
		}
		must.ErrorContains(t, f.Validate(), "unknown configuration options: bad")
	})
}

func Test_validEnvFingerprinters(t *testing.T) {
	expectedFingerprinters := []string{
		"env_aws",
		"env_azure",
		"env_gce",
		"env_digitalocean",
	}
	must.Eq(t, expectedFingerprinters, validEnvFingerprinters)
}
