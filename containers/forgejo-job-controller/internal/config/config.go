// Package config reads the controller settings from environment variables.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

const namespaceFile = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"

// Config holds every setting of the controller.
type Config struct {
	ForgejoURL       string            // API base the controller polls, e.g. http://forgejo-http.forgejo.svc:3000
	ForgejoRunnerURL string            // URL the runner pods use to reach Forgejo (defaults to ForgejoURL)
	ForgejoScope     string            // orgs/<org>, repos/<owner>/<repo>, user or admin
	ForgejoToken     string            // personal access token that can manage runners in the scope
	Namespace        string            // namespace of the job pods
	Labels           map[string]string // runs-on label -> runner image, e.g. ubuntu-latest -> docker://node:lts
	RunnerImage      string
	DindImage        string
	RunnerConfigMap  string // optional ConfigMap with config.yml, mounted at /config
	PollInterval     time.Duration
	MaxJobs          int
	JobTimeout       time.Duration
	FinishedTTL      time.Duration
	LongRunningAfter time.Duration // warn about job pods older than this, before JobTimeout kills them
	JobCPULimit      string
	JobMemoryLimit   string
	JobDiskLimit     string
	DindMTU          int
	Listen           string
	LogLevel         slog.Level
	KubeAPIURL       string // empty means in cluster
	KubeTokenFile    string
	KubeDryRun       bool
}

// Load builds the configuration from the given environment lookup.
func Load(getenv func(string) string) (Config, error) {
	c := Config{
		ForgejoURL:       strings.TrimRight(getenv("FORGEJO_URL"), "/"),
		ForgejoRunnerURL: strings.TrimRight(getenv("FORGEJO_RUNNER_URL"), "/"),
		ForgejoScope:     strings.Trim(getenv("FORGEJO_SCOPE"), "/"),
		Namespace:        getenv("NAMESPACE"),
		RunnerImage:      getenv("RUNNER_IMAGE"),
		DindImage:        getenv("DIND_IMAGE"),
		RunnerConfigMap:  getenv("RUNNER_CONFIGMAP"),
		JobCPULimit:      or(getenv("JOB_CPU_LIMIT"), "2"),
		JobMemoryLimit:   or(getenv("JOB_MEMORY_LIMIT"), "4Gi"),
		JobDiskLimit:     or(getenv("JOB_DISK_LIMIT"), "10Gi"),
		Listen:           or(getenv("LISTEN"), ":8080"),
		KubeAPIURL:       strings.TrimRight(getenv("KUBE_API_URL"), "/"),
		KubeTokenFile:    getenv("KUBE_TOKEN_FILE"),
	}

	if c.ForgejoURL == "" {
		return Config{}, errors.New("FORGEJO_URL is required")
	}
	if c.ForgejoScope == "" {
		return Config{}, errors.New("FORGEJO_SCOPE is required (orgs/<org>, repos/<owner>/<repo>, user or admin)")
	}
	if c.RunnerImage == "" || c.DindImage == "" {
		return Config{}, errors.New("RUNNER_IMAGE and DIND_IMAGE are required (pin them by tag or digest)")
	}
	if c.ForgejoRunnerURL == "" {
		c.ForgejoRunnerURL = c.ForgejoURL
	}

	token, err := loadToken(getenv)
	if err != nil {
		return Config{}, err
	}
	c.ForgejoToken = token

	labels, err := parseLabels(getenv("LABELS"))
	if err != nil {
		return Config{}, err
	}
	c.Labels = labels

	if c.Namespace == "" {
		ns, err := os.ReadFile(namespaceFile)
		if err != nil {
			return Config{}, errors.New("NAMESPACE is required outside a cluster")
		}
		c.Namespace = strings.TrimSpace(string(ns))
	}

	if c.PollInterval, err = duration(getenv("POLL_INTERVAL"), 5*time.Second); err != nil {
		return Config{}, fmt.Errorf("POLL_INTERVAL: %w", err)
	}
	if c.JobTimeout, err = duration(getenv("JOB_TIMEOUT"), 2*time.Hour); err != nil {
		return Config{}, fmt.Errorf("JOB_TIMEOUT: %w", err)
	}
	if c.FinishedTTL, err = duration(getenv("FINISHED_TTL"), 30*time.Minute); err != nil {
		return Config{}, fmt.Errorf("FINISHED_TTL: %w", err)
	}
	if c.LongRunningAfter, err = duration(getenv("LONG_RUNNING_AFTER"), 45*time.Minute); err != nil {
		return Config{}, fmt.Errorf("LONG_RUNNING_AFTER: %w", err)
	}
	if c.MaxJobs, err = integer(getenv("MAX_JOBS"), 2); err != nil {
		return Config{}, fmt.Errorf("MAX_JOBS: %w", err)
	}
	if c.MaxJobs < 1 {
		return Config{}, errors.New("MAX_JOBS must be at least 1")
	}
	if c.DindMTU, err = integer(getenv("DIND_MTU"), 0); err != nil {
		return Config{}, fmt.Errorf("DIND_MTU: %w", err)
	}
	if v := getenv("KUBE_DRY_RUN"); v != "" {
		if c.KubeDryRun, err = strconv.ParseBool(v); err != nil {
			return Config{}, fmt.Errorf("KUBE_DRY_RUN: %w", err)
		}
	}
	if v := getenv("LOG_LEVEL"); v != "" {
		if err := c.LogLevel.UnmarshalText([]byte(v)); err != nil {
			return Config{}, fmt.Errorf("LOG_LEVEL: %w", err)
		}
	}
	return c, nil
}

func loadToken(getenv func(string) string) (string, error) {
	if file := getenv("FORGEJO_TOKEN_FILE"); file != "" {
		b, err := os.ReadFile(file)
		if err != nil {
			return "", fmt.Errorf("read FORGEJO_TOKEN_FILE: %w", err)
		}
		return strings.TrimSpace(string(b)), nil
	}
	if token := getenv("FORGEJO_TOKEN"); token != "" {
		return token, nil
	}
	return "", errors.New("FORGEJO_TOKEN_FILE or FORGEJO_TOKEN is required")
}

// parseLabels reads "name=image,name2=image2". The image keeps its docker:// prefix.
func parseLabels(raw string) (map[string]string, error) {
	labels := map[string]string{}
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		name, image, ok := strings.Cut(item, "=")
		if !ok || name == "" || image == "" {
			return nil, fmt.Errorf("LABELS entry %q must look like name=docker://image", item)
		}
		labels[strings.TrimSpace(name)] = strings.TrimSpace(image)
	}
	if len(labels) == 0 {
		return nil, errors.New("LABELS is required (name=docker://image, comma separated)")
	}
	return labels, nil
}

func or(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

func duration(v string, fallback time.Duration) (time.Duration, error) {
	if v == "" {
		return fallback, nil
	}
	return time.ParseDuration(v)
}

func integer(v string, fallback int) (int, error) {
	if v == "" {
		return fallback, nil
	}
	return strconv.Atoi(v)
}
