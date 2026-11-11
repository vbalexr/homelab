// Package spawner turns waiting Forgejo jobs into one Kubernetes Job (one pod) each.
package spawner

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/vbalexr/homelab/containers/forgejo-job-controller/internal/config"
	"github.com/vbalexr/homelab/containers/forgejo-job-controller/internal/forgejo"
	"github.com/vbalexr/homelab/containers/forgejo-job-controller/internal/kube"
	"github.com/vbalexr/homelab/containers/forgejo-job-controller/internal/logging"
	"github.com/vbalexr/homelab/containers/forgejo-job-controller/internal/metrics"
)

// Metrics are the controller's own series.
type Metrics struct {
	Polls       *metrics.Counter // result
	Spawned     *metrics.Counter
	SpawnErrors *metrics.Counter // stage
	Reaped      *metrics.Counter
	Finished    *metrics.Counter // result, cause
	Waiting     *metrics.Gauge
	Active      *metrics.Gauge
	LongRunning *metrics.Gauge // active job pods older than LongRunningAfter
	OldestAge   *metrics.Gauge // age in seconds of the oldest active job pod
}

// NewMetrics registers the controller series.
func NewMetrics(r *metrics.Registry) Metrics {
	return Metrics{
		Polls:       r.NewCounter("fjc_polls_total", "Poll cycles by result.", "result"),
		Spawned:     r.NewCounter("fjc_jobs_spawned_total", "Kubernetes Jobs created for waiting Forgejo jobs."),
		SpawnErrors: r.NewCounter("fjc_spawn_errors_total", "Failed spawns by stage.", "stage"),
		Reaped:      r.NewCounter("fjc_runners_reaped_total", "Runners deleted from Forgejo after their Job finished."),
		Finished:    r.NewCounter("fjc_jobs_finished_total", "Finished job pods by result and cause.", "result", "cause"),
		Waiting:     r.NewGauge("fjc_waiting_jobs", "Forgejo jobs waiting for a runner that this controller serves."),
		Active:      r.NewGauge("fjc_active_jobs", "Kubernetes Jobs that have not finished."),
		LongRunning: r.NewGauge("fjc_long_running_jobs", "Active job pods older than LONG_RUNNING_AFTER."),
		OldestAge:   r.NewGauge("fjc_oldest_active_job_seconds", "Age of the oldest active job pod."),
	}
}

// Controller polls Forgejo and starts one job pod per waiting job.
type Controller struct {
	cfg     config.Config
	forgejo *forgejo.Client
	kube    *kube.Client
	m       Metrics
	skipped map[string]bool
	warned  map[string]bool // Jobs already reported as long running
	now     func() time.Time
	lastOK  atomic.Int64 // unix seconds of the last poll that succeeded
}

// New creates a controller.
func New(cfg config.Config, f *forgejo.Client, k *kube.Client, m Metrics) *Controller {
	return &Controller{cfg: cfg, forgejo: f, kube: k, m: m, skipped: map[string]bool{}, warned: map[string]bool{}, now: time.Now}
}

// Healthy reports whether a poll succeeded within the last three intervals (or in the last minute, if longer).
func (c *Controller) Healthy() bool {
	limit := 3 * c.cfg.PollInterval
	if limit < time.Minute {
		limit = time.Minute
	}
	last := c.lastOK.Load()
	return last != 0 && time.Since(time.Unix(last, 0)) < limit
}

// Run polls until the context ends.
func (c *Controller) Run(ctx context.Context) {
	ticker := time.NewTicker(c.cfg.PollInterval)
	defer ticker.Stop()
	for {
		if err := c.Tick(ctx); err != nil {
			logging.From(ctx).Error("poll failed", "error", err.Error())
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// Tick runs one poll cycle: list the waiting jobs, clean up finished ones and start what is missing.
func (c *Controller) Tick(ctx context.Context) error {
	ctx = logging.WithTrace(ctx, logging.NewTraceID())
	log := logging.From(ctx)

	jobs, err := c.forgejo.ListJobs(ctx)
	if err != nil {
		c.m.Polls.Inc("error")
		return fmt.Errorf("list forgejo jobs: %w", err)
	}
	var own kube.JobList
	if err := c.kube.List(ctx, kube.JobsPath(c.cfg.Namespace), url.Values{"labelSelector": {kube.LabelManagedBy + "=" + managedBy}}, &own); err != nil {
		c.m.Polls.Inc("error")
		return fmt.Errorf("list kubernetes jobs: %w", err)
	}
	c.m.Polls.Inc("ok")
	c.lastOK.Store(time.Now().Unix())

	c.reap(ctx, own.Items)
	c.checkLongRunning(ctx, own.Items)

	existing := map[string]bool{}
	active := 0
	for _, j := range own.Items {
		existing[j.Metadata.Name] = true
		if !j.Finished() {
			active++
		}
	}

	var waiting []forgejo.Job
	for _, j := range jobs {
		if j.Status != "waiting" {
			continue
		}
		if _, ok := labelFlags(c.cfg.Labels, j.RunsOn); !ok {
			c.noteSkipped(ctx, j)
			continue
		}
		waiting = append(waiting, j)
	}
	sort.Slice(waiting, func(a, b int) bool { return waiting[a].ID < waiting[b].ID })
	c.m.Waiting.Set(float64(len(waiting)))
	c.m.Active.Set(float64(active))

	for _, job := range waiting {
		name := JobName(job.Handle)
		if existing[name] {
			continue
		}
		if active >= c.cfg.MaxJobs {
			log.Info("at the parallel job limit, leaving jobs waiting", "limit", c.cfg.MaxJobs)
			break
		}
		if err := c.spawn(ctx, name, job); err != nil {
			log.Error("spawn failed", "job", job.Name, "handle", job.Handle, "error", err.Error())
			continue
		}
		active++
		c.m.Active.Set(float64(active))
	}
	return nil
}

func (c *Controller) noteSkipped(ctx context.Context, j forgejo.Job) {
	if c.skipped[j.Handle] {
		return
	}
	c.skipped[j.Handle] = true
	logging.From(ctx).Info("ignoring job: a runs-on label has no image mapping", "job", j.Name, "handle", j.Handle, "runs_on", j.RunsOn)
}

// stageError says which step of a spawn failed.
type stageError struct {
	stage string
	err   error
}

func (e *stageError) Error() string { return e.stage + ": " + e.err.Error() }
func (e *stageError) Unwrap() error { return e.err }

// spawn registers an ephemeral runner, stores its credentials in a Secret and creates the Job. Anything created
// before a failure is removed again, so a half-started job never lingers.
func (c *Controller) spawn(ctx context.Context, name string, job forgejo.Job) (err error) {
	log := logging.From(ctx).With("job", job.Name, "handle", job.Handle, "kube_job", name)
	defer func() {
		var se *stageError
		if errors.As(err, &se) {
			c.m.SpawnErrors.Inc(se.stage)
		}
	}()

	flags, _ := labelFlags(c.cfg.Labels, job.RunsOn)
	repo, rerr := c.forgejo.RepoName(ctx, job.RepoID)
	if rerr != nil {
		log.Warn("could not resolve the repository name", "repo_id", job.RepoID, "error", rerr.Error())
	}

	runner, err := c.forgejo.CreateRunner(ctx, name)
	if err != nil {
		return &stageError{"create-runner", err}
	}
	cleanupRunner := func() {
		if derr := c.forgejo.DeleteRunner(ctx, runner.ID); derr != nil {
			log.Error("could not delete runner during rollback", "runner_id", runner.ID, "error", derr.Error())
		}
	}

	secretsPath := kube.SecretsPath(c.cfg.Namespace)
	if err := c.kube.Create(ctx, secretsPath, secretManifest(name, runner), nil); err != nil {
		cleanupRunner()
		return &stageError{"create-secret", err}
	}
	cleanupSecret := func() {
		if derr := c.kube.Delete(ctx, secretsPath+"/"+name); derr != nil {
			log.Error("could not delete secret during rollback", "error", derr.Error())
		}
	}

	var created kube.Job
	jobsPath := kube.JobsPath(c.cfg.Namespace)
	if err := c.kube.Create(ctx, jobsPath, jobManifest(c.cfg, name, job, repo, runner, flags), &created); err != nil {
		cleanupSecret()
		cleanupRunner()
		return &stageError{"create-job", err}
	}

	if c.kube.DryRun() {
		log.Info("dry run: job manifest accepted by the API server", "runner_id", runner.ID)
		return nil
	}

	// The Secret goes away with the Job once the Job owns it.
	owner := map[string]any{"metadata": map[string]any{"ownerReferences": []map[string]any{{
		"apiVersion": "batch/v1", "kind": "Job", "name": name, "uid": created.Metadata.UID,
	}}}}
	if err := c.kube.MergePatch(ctx, secretsPath+"/"+name, owner); err != nil {
		if derr := c.kube.Delete(ctx, jobsPath+"/"+name); derr != nil {
			log.Error("could not delete job during rollback", "error", derr.Error())
		}
		cleanupSecret()
		cleanupRunner()
		return &stageError{"own-secret", err}
	}

	c.m.Spawned.Inc()
	log.Info("started job pod", "repo", repo, "runner_id", runner.ID, "runs_on", job.RunsOn)
	return nil
}

// reap handles every finished Job once: it works out how and why the pod ended, deletes the Forgejo runner (a pod
// that died before taking its job leaves one behind) and records the result in a log line and a metric.
func (c *Controller) reap(ctx context.Context, jobs []kube.Job) {
	log := logging.From(ctx)
	for _, j := range jobs {
		if !j.Finished() || j.Metadata.Annotations[kube.AnnotationCleaned] == "true" {
			continue
		}
		report := c.diagnose(ctx, j)

		id, err := strconv.ParseInt(j.Metadata.Annotations[kube.AnnotationRunner], 10, 64)
		if err != nil {
			log.Warn("finished job has no runner id", "kube_job", j.Metadata.Name)
			continue
		}
		if err := c.forgejo.DeleteRunner(ctx, id); err != nil {
			log.Error("could not delete runner", "kube_job", j.Metadata.Name, "runner_id", id, "error", err.Error())
			continue
		}
		patch := map[string]any{"metadata": map[string]any{"annotations": map[string]string{kube.AnnotationCleaned: "true"}}}
		if err := c.kube.MergePatch(ctx, kube.JobsPath(c.cfg.Namespace)+"/"+j.Metadata.Name, patch); err != nil {
			log.Error("could not mark job as cleaned", "kube_job", j.Metadata.Name, "error", err.Error())
			continue
		}
		c.m.Reaped.Inc()
		c.m.Finished.Inc(report.Result, report.Cause)

		fields := []any{
			"kube_job", j.Metadata.Name, "result", report.Result, "cause", report.Cause, "job_reason", report.JobReason,
			"pod_phase", report.PodPhase, "containers", report.Containers, "events", report.Events,
			"forgejo_job", j.Metadata.Annotations[kube.AnnotationJobName], "forgejo_job_id", j.Metadata.Annotations[kube.AnnotationJobID],
			"repo", j.Metadata.Annotations[kube.AnnotationRepo], "handle", j.Metadata.Annotations[kube.AnnotationHandle], "runner_id", id,
		}
		if report.Result == "failed" {
			log.Error("job pod failed", fields...)
			continue
		}
		log.Info("job pod finished", fields...)
	}
}

// checkLongRunning reports job pods that have been alive for longer than LongRunningAfter, once each, with what the
// pod is doing (a pod stuck pulling an image or waiting for the job looks different from one running a slow build).
// Kubernetes ends them at JOB_TIMEOUT anyway; this tells you before that.
func (c *Controller) checkLongRunning(ctx context.Context, jobs []kube.Job) {
	log := logging.From(ctx)
	long, oldest := 0, time.Duration(0)
	seen := map[string]bool{}
	for _, j := range jobs {
		if j.Finished() {
			continue
		}
		name := j.Metadata.Name
		seen[name] = true
		age := c.now().Sub(j.Metadata.CreationTimestamp)
		if age > oldest {
			oldest = age
		}
		if age < c.cfg.LongRunningAfter {
			continue
		}
		long++
		if c.warned[name] {
			continue
		}
		c.warned[name] = true
		fields := []any{
			"kube_job", name, "age", age.Round(time.Second).String(), "limit", c.cfg.LongRunningAfter.String(),
			"killed_at", c.cfg.JobTimeout.String(),
			"forgejo_job", j.Metadata.Annotations[kube.AnnotationJobName], "forgejo_job_id", j.Metadata.Annotations[kube.AnnotationJobID],
			"repo", j.Metadata.Annotations[kube.AnnotationRepo], "handle", j.Metadata.Annotations[kube.AnnotationHandle],
		}
		if pod, ok := c.podOf(ctx, j); ok {
			fields = append(fields, "pod_phase", pod.Status.Phase, "containers", containerStates(pod))
		}
		log.Warn("job pod has been running for a long time", fields...)
	}
	for name := range c.warned {
		if !seen[name] {
			delete(c.warned, name)
		}
	}
	c.m.LongRunning.Set(float64(long))
	c.m.OldestAge.Set(oldest.Seconds())
}
