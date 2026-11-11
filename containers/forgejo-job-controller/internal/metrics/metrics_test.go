package metrics

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExposition(t *testing.T) {
	r := &Registry{}
	c := r.NewCounter("things_total", "Things.", "kind")
	g := r.NewGauge("level", "Level.")
	h := r.NewHistogram("dur_seconds", "Duration.", []float64{0.1, 1}, "peer")
	c.Inc("a")
	c.Inc("a")
	g.Set(3)
	h.Observe(0.5, "x")

	rec := httptest.NewRecorder()
	r.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	out := rec.Body.String()
	for _, want := range []string{`things_total{kind="a"} 2`, "level 3", `dur_seconds_bucket{peer="x",le="0.1"} 0`,
		`dur_seconds_bucket{peer="x",le="1"} 1`, `dur_seconds_bucket{peer="x",le="+Inf"} 1`, `dur_seconds_count{peer="x"} 1`} {
		if !strings.Contains(out, want) {
			t.Errorf("output misses %q:\n%s", want, out)
		}
	}
}
