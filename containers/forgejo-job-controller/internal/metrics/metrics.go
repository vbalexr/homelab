// Package metrics is a small Prometheus text-format registry (counters, gauges and histograms with labels). It avoids
// pulling the Prometheus client and its dependency tree into a binary that holds cluster permissions.
package metrics

import (
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
)

type collector interface {
	write(w io.Writer)
}

// Registry holds the collectors and serves them on /metrics.
type Registry struct {
	mu         sync.Mutex
	collectors []collector
}

func (r *Registry) add(c collector) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.collectors = append(r.collectors, c)
}

// Handler serves the registry in the Prometheus text exposition format.
func (r *Registry) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		r.mu.Lock()
		defer r.mu.Unlock()
		for _, c := range r.collectors {
			c.write(w)
		}
	})
}

const keySep = "\xff"

type series struct {
	name, help, kind string
	labels           []string
	mu               sync.Mutex
	values           map[string]float64
}

func (s *series) key(values []string) string {
	if len(values) != len(s.labels) {
		panic(fmt.Sprintf("metric %s expects %d label values, got %d", s.name, len(s.labels), len(values)))
	}
	return strings.Join(values, keySep)
}

func (s *series) write(w io.Writer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s %s\n", s.name, s.help, s.name, s.kind)
	for _, k := range sortedKeys(s.values) {
		fmt.Fprintf(w, "%s%s %g\n", s.name, labelString(s.labels, k, ""), s.values[k])
	}
}

// Counter is a monotonically increasing value per label set.
type Counter struct{ s *series }

// NewCounter registers a counter.
func (r *Registry) NewCounter(name, help string, labels ...string) *Counter {
	c := &Counter{&series{name: name, help: help, kind: "counter", labels: labels, values: map[string]float64{}}}
	r.add(c.s)
	return c
}

// Inc adds one to the series with the given label values.
func (c *Counter) Inc(labelValues ...string) {
	c.s.mu.Lock()
	defer c.s.mu.Unlock()
	c.s.values[c.s.key(labelValues)]++
}

// Gauge is a value that can go up and down.
type Gauge struct{ s *series }

// NewGauge registers a gauge.
func (r *Registry) NewGauge(name, help string, labels ...string) *Gauge {
	g := &Gauge{&series{name: name, help: help, kind: "gauge", labels: labels, values: map[string]float64{}}}
	r.add(g.s)
	return g
}

// Set stores the value of the series with the given label values.
func (g *Gauge) Set(v float64, labelValues ...string) {
	g.s.mu.Lock()
	defer g.s.mu.Unlock()
	g.s.values[g.s.key(labelValues)] = v
}

// Histogram counts observations into fixed buckets.
type Histogram struct {
	name, help string
	labels     []string
	buckets    []float64
	mu         sync.Mutex
	counts     map[string][]uint64
	sums       map[string]float64
	totals     map[string]uint64
}

// NewHistogram registers a histogram with the given upper bounds (ascending).
func (r *Registry) NewHistogram(name, help string, buckets []float64, labels ...string) *Histogram {
	h := &Histogram{name: name, help: help, labels: labels, buckets: buckets,
		counts: map[string][]uint64{}, sums: map[string]float64{}, totals: map[string]uint64{}}
	r.add(h)
	return h
}

// Observe records one value in the series with the given label values.
func (h *Histogram) Observe(v float64, labelValues ...string) {
	if len(labelValues) != len(h.labels) {
		panic(fmt.Sprintf("metric %s expects %d label values, got %d", h.name, len(h.labels), len(labelValues)))
	}
	k := strings.Join(labelValues, keySep)
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.counts[k] == nil {
		h.counts[k] = make([]uint64, len(h.buckets))
	}
	for i, b := range h.buckets {
		if v <= b {
			h.counts[k][i]++
		}
	}
	h.sums[k] += v
	h.totals[k]++
}

func (h *Histogram) write(w io.Writer) {
	h.mu.Lock()
	defer h.mu.Unlock()
	fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s histogram\n", h.name, h.help, h.name)
	keys := make([]string, 0, len(h.totals))
	for k := range h.totals {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		for i, b := range h.buckets {
			fmt.Fprintf(w, "%s_bucket%s %d\n", h.name, labelString(h.labels, k, fmt.Sprintf("%g", b)), h.counts[k][i])
		}
		fmt.Fprintf(w, "%s_bucket%s %d\n", h.name, labelString(h.labels, k, "+Inf"), h.totals[k])
		fmt.Fprintf(w, "%s_sum%s %g\n", h.name, labelString(h.labels, k, ""), h.sums[k])
		fmt.Fprintf(w, "%s_count%s %d\n", h.name, labelString(h.labels, k, ""), h.totals[k])
	}
}

func sortedKeys(m map[string]float64) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// labelString renders {a="x",b="y"}, with an extra le label when le is not empty.
func labelString(names []string, key, le string) string {
	var parts []string
	if len(names) > 0 {
		for i, v := range strings.Split(key, keySep) {
			parts = append(parts, fmt.Sprintf(`%s="%s"`, names[i], escape(v)))
		}
	}
	if le != "" {
		parts = append(parts, fmt.Sprintf(`le="%s"`, le))
	}
	if len(parts) == 0 {
		return ""
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func escape(v string) string {
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`).Replace(v)
}
