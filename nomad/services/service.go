package services

import (
	"github.com/armon/go-metrics"
	"sync"
	"time"
)

var cache sync.Map

func New[T Service](key string) (t *Service) {
	svc, ok := cache.Load(key)

	if ok {
		return svc.(Service).Copy()
	}

	svc = new(Service)
	svc.(Service).Init()
	svc, _ = cache.LoadOrStore(key, svc)
	return svc.(Service).Copy()
}

type Service interface {
	Init()
	Copy() *Service
}

type ServiceBase struct {
	Options map[string]*Options
}

type Options struct {
	MetricKeys           []string
	RequiredCapabilities []string
}

func (opts *Options) EmitMetrics() {
	if len(opts.MetricKeys) == 0 {
		return
	}

	metrics.MeasureSince(opts.MetricKeys, time.Now())
}
