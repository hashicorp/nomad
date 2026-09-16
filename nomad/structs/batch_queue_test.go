// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"testing"
	"time"

	"github.com/shoenig/test/must"
)

func TestBatchQueueConfig_Copy(t *testing.T) {
	var orig *BatchQueueConfig
	must.Nil(t, orig.Copy())

	orig = &BatchQueueConfig{
		DynamicPriority: &DynamicQueueConfig{
			TenantType:   "test-type", // string
			CalcInterval: time.Second, // duration / int64
		},
		Fifo: &FifoQueueConfig{},
	}
	cp := orig.Copy()

	must.NotNil(t, cp.Fifo, must.Sprint("missing fifo"))
	must.NotNil(t, cp.DynamicPriority, must.Sprint("missing dynamic priority"))
	must.Eq(t, "test-type", cp.DynamicPriority.TenantType, must.Sprint("string mismatch"))
	must.Eq(t, time.Second, cp.DynamicPriority.CalcInterval, must.Sprint("duration mismatch"))
	must.Eq(t, string(orig.Hash()), string(cp.Hash()), must.Sprint("Hash mismatch"))

	cp.DynamicPriority.TenantType = "copy-changed"
	must.NotEq(t, "copy-changed", orig.DynamicPriority.TenantType)
}

func TestBatchQueueConfig_IsEnabled(t *testing.T) {
	var bq *BatchQueueConfig
	must.False(t, bq.IsEnabled(), must.Sprint("nil config is not enabled"))

	bq = &BatchQueueConfig{}
	must.False(t, bq.IsEnabled(), must.Sprint("empty config is not enabled"))

	bq = &BatchQueueConfig{
		DynamicPriority: &DynamicQueueConfig{},
	}
	must.False(t, bq.IsEnabled(), must.Sprint("failing validation is not enabled"))
}

func TestBatchQueueConfig_Type(t *testing.T) {
	t.Run("dynamic", func(t *testing.T) {
		c := &BatchQueueConfig{
			DynamicPriority: &DynamicQueueConfig{
				TenantType: TenantTypeNamespace,
			},
		}
		must.Eq(t, BatchQueueTypeDynamic, c.Type())
	})
	t.Run("fifo", func(t *testing.T) {
		c := &BatchQueueConfig{
			Fifo: &FifoQueueConfig{},
		}
		must.Eq(t, BatchQueueTypeFifo, c.Type())
	})
	t.Run("unset", func(t *testing.T) {
		c := &BatchQueueConfig{
			// no queue type specified
		}
		must.Eq(t, BatchQueueTypeUnset, c.Type())
	})
}

func TestBatchQueueConfig_Validate(t *testing.T) {
	cases := []struct {
		name   string
		config BatchQueueConfig
		err    string
	}{
		{
			name: "no type",
			err:  "missing batch queue type",
		},
		{
			name: "invalid dynamic queue config",
			config: BatchQueueConfig{
				DynamicPriority: &DynamicQueueConfig{},
			},
			err: "must be specified",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.config.Validate()
			if tc.err != "" {
				must.ErrorContains(t, err, tc.err)
			} else {
				must.NoError(t, err)
			}
		})
	}
}

func TestBatchQueue_DynamicQueueConfig_Validate(t *testing.T) {
	cases := []struct {
		name   string
		config DynamicQueueConfig
		err    string
	}{
		{
			name: "missing tenant type",
			config: DynamicQueueConfig{
				TenantType: "",
			},
			err: "tenant type must be specified",
		},
		{
			name: "invalid tenant type",
			config: DynamicQueueConfig{
				TenantType: "foo",
			},
			err: "unsupported tenant type: \"foo\"",
		},
		{
			name: "empty metadata key errors",
			config: DynamicQueueConfig{
				TenantType: TenantTypeMetadata,
			},
			err: "metadata key must be specified",
		},
		{
			name: "dynamicPriority - zero calc interval",
			config: DynamicQueueConfig{
				TenantType: TenantTypeNamespace,

				CalcInterval: 0,
				HalfLife:     1 * time.Second,
			},
			err: "calc_interval must be greater than zero",
		},
		{
			name: "dynamicPriority - zero half life",
			config: DynamicQueueConfig{
				TenantType: TenantTypeNamespace,

				CalcInterval: 1 * time.Second,
				HalfLife:     0,
			},
			err: "half_life must be greater than zero",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.config.Validate()
			if tc.err != "" {
				must.ErrorContains(t, err, tc.err)
			} else {
				must.NoError(t, err)
			}
		})
	}
}
