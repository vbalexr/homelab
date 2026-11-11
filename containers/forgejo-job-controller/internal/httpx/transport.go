// Package httpx wraps an HTTP transport so every outbound call is logged with the calling service, the peer, the
// duration and the trace ID, measured in a histogram, and carries the trace ID in a W3C traceparent header.
package httpx

import (
	"net/http"
	"strconv"
	"time"

	"github.com/vbalexr/homelab/containers/forgejo-job-controller/internal/logging"
	"github.com/vbalexr/homelab/containers/forgejo-job-controller/internal/metrics"
)

// Transport is an http.RoundTripper that instruments calls to one peer service.
type Transport struct {
	Peer     string
	Base     http.RoundTripper
	Duration *metrics.Histogram // labels: peer, method, status
}

// RoundTrip sends the request and records the call.
func (t Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	trace := logging.TraceID(ctx)
	req = req.Clone(ctx)
	if trace != "" {
		req.Header.Set("traceparent", "00-"+trace+"-"+logging.NewTraceID()[:16]+"-01")
	}

	start := time.Now()
	resp, err := t.Base.RoundTrip(req)
	elapsed := time.Since(start)

	status := "error"
	if err == nil {
		status = strconv.Itoa(resp.StatusCode)
	}
	if t.Duration != nil {
		t.Duration.Observe(elapsed.Seconds(), t.Peer, req.Method, status)
	}
	log := logging.From(ctx).With("peer", t.Peer, "method", req.Method, "path", req.URL.Path,
		"status", status, "duration_ms", elapsed.Milliseconds(), "caller", logging.Service)
	if err != nil {
		log.Error("outbound call failed", "error", err.Error())
		return nil, err
	}
	log.Info("outbound call")
	return resp, nil
}
