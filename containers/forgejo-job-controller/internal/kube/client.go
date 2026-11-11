// Package kube is a minimal Kubernetes REST client: create, get, list, patch and delete on namespaced objects.
package kube

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	tokenFile = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	caFile    = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
)

// Client talks to the Kubernetes API with a bearer token that is re-read on every request (it rotates).
type Client struct {
	baseURL   string
	tokenFile string
	dryRun    bool
	http      *http.Client
}

// Settings selects the API endpoint. An empty APIURL means in cluster.
type Settings struct {
	APIURL    string
	TokenFile string
	DryRun    bool
}

// NewTransport builds the base transport: in cluster it trusts the service account CA.
func NewTransport(s Settings) (*http.Transport, error) {
	t := &http.Transport{TLSHandshakeTimeout: 10 * time.Second, MaxIdleConns: 4, IdleConnTimeout: 90 * time.Second}
	if s.APIURL != "" {
		return t, nil
	}
	ca, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("read service account CA: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(ca) {
		return nil, fmt.Errorf("no certificate in %s", caFile)
	}
	t.TLSClientConfig = &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}
	return t, nil
}

// New creates a client around an HTTP client built from NewTransport (optionally wrapped for logging).
func New(s Settings, hc *http.Client) (*Client, error) {
	c := &Client{baseURL: s.APIURL, tokenFile: s.TokenFile, dryRun: s.DryRun, http: hc}
	if c.baseURL == "" {
		host, port := os.Getenv("KUBERNETES_SERVICE_HOST"), os.Getenv("KUBERNETES_SERVICE_PORT")
		if host == "" || port == "" {
			return nil, fmt.Errorf("KUBE_API_URL is required outside a cluster")
		}
		c.baseURL = "https://" + hostPort(host, port)
		if c.tokenFile == "" {
			c.tokenFile = tokenFile
		}
	}
	return c, nil
}

func hostPort(host, port string) string {
	if strings.Contains(host, ":") {
		return "[" + host + "]:" + port
	}
	return host + ":" + port
}

// APIError is a non-2xx answer from the API server.
type APIError struct {
	Method, Path string
	Status       int
	Body         string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("kubernetes %s %s: status %d: %s", e.Method, e.Path, e.Status, e.Body)
}

// Get reads one object. It returns false when the object does not exist.
func (c *Client) Get(ctx context.Context, path string, out any) (bool, error) {
	err := c.do(ctx, http.MethodGet, path, "", nil, out)
	if apiErr, ok := err.(*APIError); ok && apiErr.Status == http.StatusNotFound {
		return false, nil
	}
	return err == nil, err
}

// List reads the objects of a collection. query may carry labelSelector and fieldSelector.
func (c *Client) List(ctx context.Context, path string, query url.Values, out any) error {
	return c.do(ctx, http.MethodGet, path, query.Encode(), nil, out)
}

// Create posts a new object and decodes the stored object into out.
func (c *Client) Create(ctx context.Context, path string, body, out any) error {
	return c.do(ctx, http.MethodPost, path, c.writeQuery(), body, out)
}

// MergePatch applies a JSON merge patch to an object.
func (c *Client) MergePatch(ctx context.Context, path string, body any) error {
	return c.do(ctx, http.MethodPatch, path, c.writeQuery(), body, nil)
}

// Delete removes an object and, in the background, what it owns. A missing object is not an error.
func (c *Client) Delete(ctx context.Context, path string) error {
	q := url.Values{"propagationPolicy": {"Background"}}
	if c.dryRun {
		q.Set("dryRun", "All")
	}
	err := c.do(ctx, http.MethodDelete, path, q.Encode(), nil, nil)
	if apiErr, ok := err.(*APIError); ok && apiErr.Status == http.StatusNotFound {
		return nil
	}
	return err
}

// DryRun reports whether writes are only validated by the API server.
func (c *Client) DryRun() bool { return c.dryRun }

func (c *Client) writeQuery() string {
	if c.dryRun {
		return "dryRun=All"
	}
	return ""
}

func (c *Client) do(ctx context.Context, method, path, query string, body, out any) error {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(b)
	}
	target := c.baseURL + path
	if query != "" {
		target += "?" + query
	}
	req, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return err
	}
	if c.tokenFile != "" {
		token, err := os.ReadFile(c.tokenFile)
		if err != nil {
			return fmt.Errorf("read token: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(string(token)))
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		if method == http.MethodPatch {
			req.Header.Set("Content-Type", "application/merge-patch+json")
		} else {
			req.Header.Set("Content-Type", "application/json")
		}
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return &APIError{Method: method, Path: path, Status: resp.StatusCode, Body: string(bytes.TrimSpace(data))}
	}
	if out == nil || len(bytes.TrimSpace(data)) == 0 {
		return nil
	}
	return json.Unmarshal(data, out)
}
