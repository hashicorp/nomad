package structs

import (
	"bytes"
	"errors"
	"fmt"
	"time"
)

type (
	BatchQueueType   string
	BatchQueueTenant string
)

const (
	TenantTypeMetadata  BatchQueueTenant = "metadata"
	TenantTypeNamespace BatchQueueTenant = "namespace"

	BatchQueueTypeDynamic     BatchQueueType = "dynamic_priority"
	BatchQueueTypeFifo        BatchQueueType = "fifo"
	BatchQueueTypePassthrough BatchQueueType = "passthrough"
	BatchQueueTypeUnset       BatchQueueType = "unset"
)

type BatchQueueConfig struct {
	DynamicPriority *DynamicQueueConfig
	Fifo            *FifoQueueConfig
}

func (bq *BatchQueueConfig) IsEnabled() bool {
	return bq != nil && bq.Validate() == nil
}

// Type returns the batch queue type. Use it in switch statements to detemrine
// which config to use (DynamicPriority or Fifo).
func (bq *BatchQueueConfig) Type() BatchQueueType {
	if bq.DynamicPriority != nil {
		return BatchQueueTypeDynamic
	}
	if bq.Fifo != nil {
		return BatchQueueTypeFifo
	}
	return BatchQueueTypeUnset
}

func (b *BatchQueueConfig) Validate() error {
	switch b.Type() {

	case BatchQueueTypeDynamic:
		return b.DynamicPriority.Validate()

	case BatchQueueTypeFifo:
	case BatchQueueTypePassthrough:

	case BatchQueueTypeUnset:
		return errors.New("missing batch queue type (dynamic_priority or fifo)")
	default:
		return fmt.Errorf("unsupported batch queue type: %q", b.Type())
	}

	return nil
}

func (bq *BatchQueueConfig) Copy() *BatchQueueConfig {
	if bq == nil {
		return nil
	}
	var dyn DynamicQueueConfig
	var fifo FifoQueueConfig
	if bq.DynamicPriority != nil {
		dyn = *bq.DynamicPriority
	}
	if bq.Fifo != nil {
		fifo = *bq.Fifo
	}
	return &BatchQueueConfig{
		DynamicPriority: &dyn,
		Fifo:            &fifo,
	}
}

func (bq *BatchQueueConfig) Hash() []byte {
	if bq == nil {
		return []byte{}
	}
	var buf bytes.Buffer
	if bq.DynamicPriority != nil {
		buf.WriteString(fmt.Sprintf("dynamic_priority_%v", bq.DynamicPriority))
	}
	if bq.Fifo != nil {
		buf.WriteString("fifo")
	}
	return buf.Bytes()
}

// DynamicQueueConfig configures a dynamic priority queue for a node pool.
// Refer to to api.DynamicQueueConfig for detailed doc comments.
type DynamicQueueConfig struct {
	TenantType  BatchQueueTenant
	MetadataKey string

	CalcInterval time.Duration
	AgeWeight    int
	MaxAge       time.Duration
	HalfLife     time.Duration
	SizeWeight   int
	MaxSize      int
	UsageWeight  int
}

func (qc *DynamicQueueConfig) Validate() error {
	switch qc.TenantType {
	case TenantTypeNamespace:
	case TenantTypeMetadata:
		if qc.MetadataKey == "" {
			return errors.New("metadata key must be specified if using metadata tenency")
		}
	case "":
		return errors.New("tenant type must be specified if using dynamic priority queue")
	default:
		return fmt.Errorf("unsupported tenant type: %q", qc.TenantType)
	}

	if qc.CalcInterval <= 0 {
		return errors.New("calc_interval must be greater than zero")
	}
	if qc.HalfLife <= 0 {
		return errors.New("half_life must be greater than zero")
	}

	return nil
}

type FifoQueueConfig struct{}
