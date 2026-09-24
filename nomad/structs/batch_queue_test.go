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
			CalcInterval:    time.Second,
			TenantFairshare: TenantFairshareConfig{TenantType: "test-type"},
		},
		Fifo: &FifoQueueConfig{},
	}

	cp := orig.Copy()

	must.NotNil(t, cp.Fifo, must.Sprint("missing fifo"))
	must.NotNil(t, cp.DynamicPriority, must.Sprint("missing dynamic priority"))
	must.Eq(t, BatchQueueTenant("test-type"), cp.DynamicPriority.TenantFairshare.TenantType, must.Sprint("string mismatch"))
	must.Eq(t, time.Second, cp.DynamicPriority.CalcInterval, must.Sprint("duration mismatch"))
	must.Eq(t, string(orig.Hash()), string(cp.Hash()), must.Sprint("Hash mismatch"))

	cp.DynamicPriority.TenantFairshare.TenantType = "copy-changed"
	must.Eq(t, BatchQueueTenant("test-type"), orig.DynamicPriority.TenantFairshare.TenantType)
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
				TenantFairshare: TenantFairshareConfig{TenantType: TenantTypeNamespace},
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
	mkConf := func(tenantType BatchQueueTenant, metadataKey string, calcInterval time.Duration) DynamicQueueConfig {
		return DynamicQueueConfig{
			CalcInterval:    calcInterval,
			TenantFairshare: TenantFairshareConfig{TenantType: tenantType, MetadataKey: metadataKey},
		}
	}

	cases := []struct {
		name   string
		config DynamicQueueConfig
		err    string
	}{
		{
			name:   "missing tenant type",
			config: mkConf("", "", 0),
			err:    "tenant type must be specified",
		},
		{
			name:   "invalid tenant type",
			config: mkConf("foo", "", 0),
			err:    "unsupported tenant type: \"foo\"",
		},
		{
			name:   "empty metadata key errors",
			config: mkConf(TenantTypeMetadata, "", 0),
			err:    "metadata key must be specified",
		},
		{
			name:   "zero calc interval",
			config: mkConf(TenantTypeNamespace, "", 0),
			err:    "calc_interval must be greater than zero",
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
