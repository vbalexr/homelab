// Package forgejo is the part of the Forgejo API the controller needs: waiting jobs and ephemeral runners.
package forgejo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Client talks to the Forgejo API in one scope (orgs/<org>, repos/<owner>/<repo>, user or admin).
type Client struct {
	baseURL string
	scope   string
	token   string
	http    *http.Client
}

// New creates a client. baseURL is the Forgejo root without /api/v1.
func New(baseURL, scope, token string, hc *http.Client) *Client {
	return &Client{baseURL: baseURL, scope: scope, token: token, http: hc}
}

// Job is a workflow job as listed by the runners/jobs endpoint.
type Job struct {
	ID      int64    `json:"id"`
	Attempt int64    `json:"attempt"`
	Handle  string   `json:"handle"`
	RepoID  int64    `json:"repo_id"`
	OwnerID int64    `json:"owner_id"`
	Name    string   `json:"name"`
	Needs   []string `json:"needs"`
	RunsOn  []string `json:"runs_on"`
	TaskID  int64    `json:"task_id"`
	Status  string   `json:"status"`
}

// Runner is a registered runner. Token is only returned on creation.
type Runner struct {
	ID    int64  `json:"id"`
	UUID  string `json:"uuid"`
	Token string `json:"token"`
}

// APIError is a non-2xx answer from Forgejo.
type APIError struct {
	Method, Path string
	Status       int
	Body         string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("forgejo %s %s: status %d: %s", e.Method, e.Path, e.Status, e.Body)
}

// ListJobs returns the jobs that are waiting or running in the scope.
func (c *Client) ListJobs(ctx context.Context) ([]Job, error) {
	var jobs []Job
	if err := c.do(ctx, http.MethodGet, "/actions/runners/jobs", nil, &jobs); err != nil {
		return nil, err
	}
	return jobs, nil
}

// CreateRunner registers an ephemeral runner: Forgejo removes it after one job.
func (c *Client) CreateRunner(ctx context.Context, name string) (Runner, error) {
	var r Runner
	body := map[string]any{"name": name, "ephemeral": true}
	if err := c.do(ctx, http.MethodPost, "/actions/runners", body, &r); err != nil {
		return Runner{}, err
	}
	if r.UUID == "" || r.Token == "" {
		return Runner{}, fmt.Errorf("forgejo returned a runner without uuid or token (id %d)", r.ID)
	}
	return r, nil
}

// DeleteRunner removes a runner. A runner that is already gone is not an error.
func (c *Client) DeleteRunner(ctx context.Context, id int64) error {
	err := c.do(ctx, http.MethodDelete, fmt.Sprintf("/actions/runners/%d", id), nil, nil)
	if apiErr, ok := err.(*APIError); ok && apiErr.Status == http.StatusNotFound {
		return nil
	}
	return err
}

// RepoName returns owner/name of a repository by ID, for log messages and annotations.
func (c *Client) RepoName(ctx context.Context, id int64) (string, error) {
	var repo struct {
		FullName string `json:"full_name"`
	}
	if err := c.doAt(ctx, http.MethodGet, fmt.Sprintf("/api/v1/repositories/%d", id), nil, &repo); err != nil {
		return "", err
	}
	return repo.FullName, nil
}

func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	return c.doAt(ctx, method, "/api/v1/"+c.scope+path, body, out)
}

func (c *Client) doAt(ctx context.Context, method, full string, body, out any) error {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+full, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "token "+c.token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return &APIError{Method: method, Path: full, Status: resp.StatusCode, Body: string(bytes.TrimSpace(data))}
	}
	if out == nil || len(bytes.TrimSpace(data)) == 0 {
		return nil
	}
	return json.Unmarshal(data, out)
}
