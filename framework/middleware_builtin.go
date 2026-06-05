package framework

import (
	"fmt"
	"sync"
	"time"
)

func LoggingMiddleware() Middleware {
	return func(next Handler) Handler {
		return func(ctx *ActorContext, msg Message) {
			log := ctx.System().Logger()
			addr := ctx.Self().Address()
			start := time.Now()
			log.Info("message in", "actor", addr, "type", fmt.Sprintf("%T", msg))
			next(ctx, msg)
			log.Info("message out", "actor", addr, "dur", time.Since(start).String())
		}
	}
}

type MetricsRegistry struct {
	mu      sync.Mutex
	counts  map[string]int64
	totalNs map[string]int64
}

func NewMetricsRegistry() *MetricsRegistry {
	return &MetricsRegistry{
		counts:  make(map[string]int64),
		totalNs: make(map[string]int64),
	}
}

func (r *MetricsRegistry) record(actor string, d time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.counts[actor]++
	r.totalNs[actor] += d.Nanoseconds()
}

func (r *MetricsRegistry) Count(actor string) int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.counts[actor]
}

func (r *MetricsRegistry) AverageLatency(actor string) time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.counts[actor] == 0 {
		return 0
	}
	return time.Duration(r.totalNs[actor] / r.counts[actor])
}

func MetricsMiddleware(reg *MetricsRegistry) Middleware {
	return func(next Handler) Handler {
		return func(ctx *ActorContext, msg Message) {
			start := time.Now()
			next(ctx, msg)
			reg.record(ctx.Self().Address(), time.Since(start))
		}
	}
}
