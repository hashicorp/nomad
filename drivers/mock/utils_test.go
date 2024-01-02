// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package mock

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/stretchr/testify/require"
)

func TestParseDuration(t *testing.T) {
	ci.Parallel(t)

	t.Run("valid case", func(t *testing.T) {
		v, err := parseDuration("10m")
		require.NoError(t, err)
		require.Equal(t, 10*time.Minute, v)
	})

	t.Run("invalid case", func(t *testing.T) {
		v, err := parseDuration("10")
		require.Error(t, err)
		require.Equal(t, time.Duration(0), v)
	})

	t.Run("empty case", func(t *testing.T) {
		v, err := parseDuration("")
		require.NoError(t, err)
		require.Equal(t, time.Duration(0), v)
	})

}
