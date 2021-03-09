package statsd

import (
	"math"
	"sync"
	"sync/atomic"
)

/*
Those are metrics type that can be aggregated on the client side:
  - Gauge
  - Count
  - Set
*/

type countMetric struct {
	value int64
	name  string
	tags  []string
	rate  float64
}

func newCountMetric(name string, value int64, tags []string, rate float64) *countMetric {
	return &countMetric{
		value: value,
		name:  name,
		tags:  tags,
		rate:  rate,
	}
}

func (c *countMetric) sample(v int64) {
	atomic.AddInt64(&c.value, v)
}

func (c *countMetric) flushUnsafe() metric {
	return metric{
		metricType: count,
		name:       c.name,
		tags:       c.tags,
		rate:       c.rate,
		ivalue:     c.value,
	}
}

// Gauge

type gaugeMetric struct {
	value uint64
	name  string
	tags  []string
	rate  float64
}

func newGaugeMetric(name string, value float64, tags []string, rate float64) *gaugeMetric {
	return &gaugeMetric{
		value: math.Float64bits(value),
		name:  name,
		tags:  tags,
		rate:  rate,
	}
}

func (g *gaugeMetric) sample(v float64) {
	atomic.StoreUint64(&g.value, math.Float64bits(v))
}

func (g *gaugeMetric) flushUnsafe() metric {
	return metric{
		metricType: gauge,
		name:       g.name,
		tags:       g.tags,
		rate:       g.rate,
		fvalue:     math.Float64frombits(g.value),
	}
}

// Set

type setMetric struct {
	data map[string]struct{}
	name string
	tags []string
	rate float64
	sync.Mutex
}

func newSetMetric(name string, value string, tags []string, rate float64) *setMetric {
	set := &setMetric{
		data: map[string]struct{}{},
		name: name,
		tags: tags,
		rate: rate,
	}
	set.data[value] = struct{}{}
	return set
}

func (s *setMetric) sample(v string) {
	s.Lock()
	defer s.Unlock()
	s.data[v] = struct{}{}
}

// Sets are aggregated on the agent side too. We flush the keys so a set from
// multiple application can be correctly aggregated on the agent side.
func (s *setMetric) flushUnsafe() []metric {
	if len(s.data) == 0 {
		return nil
	}

	metrics := make([]metric, len(s.data))
	i := 0
	for value := range s.data {
		metrics[i] = metric{
			metricType: set,
			name:       s.name,
			tags:       s.tags,
			rate:       s.rate,
			svalue:     value,
		}
		i++
	}
	return metrics
}
