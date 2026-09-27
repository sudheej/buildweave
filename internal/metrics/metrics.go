package metrics

import (
	"fmt"
	"strings"
	"sync"
)

type Registry struct {
	mu  sync.Mutex
	c   map[string]uint64
	dur float64
}

func New() *Registry                          { return &Registry{c: map[string]uint64{}} }
func (r *Registry) Inc(name string)           { r.mu.Lock(); r.c[name]++; r.mu.Unlock() }
func (r *Registry) Add(name string, n uint64) { r.mu.Lock(); r.c[name] += n; r.mu.Unlock() }
func (r *Registry) Observe(seconds float64)   { r.mu.Lock(); r.dur += seconds; r.mu.Unlock() }
func (r *Registry) Get(name string) uint64    { r.mu.Lock(); defer r.mu.Unlock(); return r.c[name] }
func (r *Registry) Prometheus() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	names := []string{"queries_total", "queries_reused_total", "queries_recomputed_total", "queries_retained_total", "queries_invalidated_total", "singleflight_joins_total", "cas_hits_total", "cas_misses_total", "shadow_build_divergences_total"}
	var b strings.Builder
	fmt.Fprintf(&b, "verification_duration_seconds_sum %g\n", r.dur)
	for _, n := range names {
		fmt.Fprintf(&b, "%s %d\n", n, r.c[n])
	}
	return b.String()
}
