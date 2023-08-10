// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package indexer

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func Test_IndexBuilder_Time(t *testing.T) {
	builder := &IndexBuilder{}
	testTime := time.Date(1987, time.April, 13, 8, 3, 0, 0, time.UTC)
	builder.Time(testTime)
	require.Equal(t, []byte{0, 0, 0, 0, 32, 128, 155, 180}, builder.Bytes())
}
