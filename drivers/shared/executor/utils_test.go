// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package executor

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUtils_IsolationMode(t *testing.T) {
	private := IsolationModePrivate
	host := IsolationModeHost
	blank := ""

	for _, tc := range []struct {
		plugin, task, exp string
	}{
		{plugin: private, task: private, exp: private},
		{plugin: private, task: host, exp: host},
		{plugin: private, task: blank, exp: private}, // default to private

		{plugin: host, task: private, exp: private},
		{plugin: host, task: host, exp: host},
		{plugin: host, task: blank, exp: host}, // default to host
	} {
		result := IsolationMode(tc.plugin, tc.task)
		require.Equal(t, tc.exp, result)
	}
}
