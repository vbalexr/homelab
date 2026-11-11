# forgejo-job-controller

Starts **one Kubernetes pod per Forgejo Actions job**. It polls Forgejo for waiting jobs; for each new one it registers an
ephemeral runner, stores the runner credentials in a Secret and creates a Job whose pod runs `forgejo-runner one-job --handle <handle>`
next to a Docker-in-Docker sidecar. The pod ends with its job.

Go, standard library only (no client-go, no Prometheus client), a static binary in a `scratch` image.

## How it works

1. `GET /api/v1/<scope>/actions/runners/jobs`, keep jobs with status `waiting` whose `runs_on` labels all have an image in `LABELS`
   (others belong to another runner and are ignored).
2. The Kubernetes Job name is `fj-` + a hash of the job `handle`, so a handle that already has a Job is never started twice. At most `MAX_JOBS` Jobs run at once.
3. `POST .../actions/runners` with `{"ephemeral": true}` returns the runner UUID and token. They go into a Secret named like the Job
   (owned by the Job, so it disappears with it). Forgejo removes the runner itself after the job it took.
4. The Job has one pod: `dind` (native sidecar, privileged, Unix socket only, no TCP listener) and `runner` (non-root, read-only root filesystem,
   no service account token). `backoffLimit: 0`, `activeDeadlineSeconds: JOB_TIMEOUT`, `ttlSecondsAfterFinished: FINISHED_TTL`.
5. A finished Job whose pod never took the job leaves its runner behind; the controller deletes that runner once (annotation `forgejo.vbalex.com/runner-cleaned`).
6. If any step of a spawn fails, what was created before it is removed again.

Needs Forgejo 15+ and runner 12.8+ (the `handle` field and `one-job --handle`).

## Configuration (environment)

| Variable | Default | Meaning |
|---|---|---|
| `FORGEJO_URL` | required | Forgejo base URL the controller polls |
| `FORGEJO_RUNNER_URL` | `FORGEJO_URL` | URL the runner pods use (`actions/checkout` still clones from Forgejo's `ROOT_URL`) |
| `FORGEJO_SCOPE` | required | `orgs/<org>`, `repos/<owner>/<repo>`, `user` or `admin` |
| `FORGEJO_TOKEN_FILE` / `FORGEJO_TOKEN` | required | Personal access token that can manage runners in the scope |
| `LABELS` | required | `runs-on` label to image, comma separated: `ubuntu-latest=docker://node:22-bookworm,docker=docker://...` |
| `RUNNER_IMAGE`, `DIND_IMAGE` | required | Pin them by tag or digest |
| `RUNNER_CONFIGMAP` | none | ConfigMap with `config.yml`, mounted at `/config` and passed as `-c` |
| `NAMESPACE` | service account namespace | Where the Jobs and Secrets are created |
| `POLL_INTERVAL` | `5s` | |
| `MAX_JOBS` | `2` | Parallel job pods |
| `JOB_TIMEOUT` | `2h` | Pod deadline, includes the wait for the job |
| `LONG_RUNNING_AFTER` | `45m` | A job pod older than this is logged once as `job pod has been running for a long time` (with its container states) and counted in `fjc_long_running_jobs`. Kubernetes still ends it at `JOB_TIMEOUT` |
| `FINISHED_TTL` | `30m` | How long finished Jobs stay. A job still `waiting` after that is started again |
| `JOB_CPU_LIMIT`, `JOB_MEMORY_LIMIT`, `JOB_DISK_LIMIT` | `2`, `4Gi`, `10Gi` | Limits of the dind container (the whole job runs inside it) and the size of `/var/lib/docker` |
| `DIND_MTU` | unset | Set it if the pod network MTU is below 1500 |
| `LISTEN` | `:8080` | `/metrics`, `/healthz` |
| `LOG_LEVEL` | `info` | |
| `KUBE_API_URL`, `KUBE_TOKEN_FILE`, `KUBE_DRY_RUN` | in cluster | Local testing: point at `kubectl proxy`; `KUBE_DRY_RUN=true` only validates manifests |

Flag `-once` runs one poll cycle and exits.

Runner `config.yml` (the file in `RUNNER_CONFIGMAP`; it must not contain `server.connections`, the connection comes from flags):

```yaml
log:
  level: info
runner:
  timeout: 1h
container:
  # jobs get the pod's own dind daemon: root-equivalent inside the pod, which dies with the job
  docker_host: automount
  privileged: false
  valid_volumes: []
```

## Kubernetes access

A Role in the job namespace is enough: `batch/jobs` create, get, list, patch, delete; `secrets` create, get, patch, delete; `pods` and `events` get, list (read-only, to report why a pod failed).
The pods it creates run with `automountServiceAccountToken: false`. The dind sidecar is privileged, so the namespace needs
`pod-security.kubernetes.io/enforce: privileged`; add a NetworkPolicy that keeps job pods away from the rest of the cluster.

## Observability

- JSON logs on stdout. Every line has `service` and `trace_id` (one trace per poll cycle). Every call to Forgejo or the Kubernetes API is logged with
  `peer`, `caller`, `method`, `path`, `status` and `duration_ms`, and carries a W3C `traceparent` header.
- Prometheus on `/metrics`: `fjc_polls_total{result}`, `fjc_jobs_spawned_total`, `fjc_spawn_errors_total{stage}`, `fjc_runners_reaped_total`,
  `fjc_jobs_finished_total{result,cause}`, `fjc_long_running_jobs`, `fjc_oldest_active_job_seconds`, `fjc_waiting_jobs`, `fjc_active_jobs`, `fjc_outbound_call_duration_seconds{peer,method,status}`.
- **Why a job pod failed:** when a job pod ends, one log line `job pod failed` (level error) or `job pod finished` carries `cause`, `job_reason`,
  `pod_phase`, the state of every container (`runner: terminated exit=137 reason=OOMKilled`), the last warning events of the pod, and `repo`, `forgejo_job`,
  `forgejo_job_id` and `handle` to find the run in Forgejo. Causes: `oom-killed-<container>`, `runner-exit-<code>`, `dind-failed`, `image-pull`, `container-config`,
  `unschedulable`, `deadlineexceeded`, `backofflimitexceeded`. Counted in `fjc_jobs_finished_total{result,cause}`.
  Alert on `increase(fjc_jobs_finished_total{result="failed"}[15m]) > 0` and on the log line in Loki: `{namespace="forgejo-runners"} | json | msg="job pod failed"`.
- **Pods that run too long:** `LONG_RUNNING_AFTER` (45m) warns early, `JOB_TIMEOUT` (2h) kills. The warning shows what the pod is doing, so a stuck image pull, a pod waiting for its job and a slow build look different. Alert on `fjc_long_running_jobs > 0`.
- `/healthz` fails when no poll succeeded for three intervals (at least one minute).

## Develop

```bash
podman run --rm -v "$PWD":/src:Z -w /src docker.io/library/golang:1.26-alpine sh -c 'go vet ./... && go test ./...'
docker build -t ghcr.io/vbalexr/forgejo-job-controller:dev .
```

CI (`.github/workflows/build-forgejo-job-controller.yml`) runs vet and tests, then pushes `ghcr.io/vbalexr/forgejo-job-controller:<YYYYMMDD-HHMM>` and `:latest`.

## Known limits

- **Workflow failures are not pod failures.** A failing step (tests, build) ends the runner normally, so the Kubernetes Job succeeds. Step output and the red/green result are in
  the Forgejo UI (run page); the pod logs in Loki only hold runner and Docker daemon lines. This controller reports infrastructure failures, not step failures.
- A pod stuck pulling an image or unschedulable is only reported when its deadline (`JOB_TIMEOUT`) hits. Alert on `kube_pod_container_status_waiting_reason{namespace="forgejo-runners",reason=~"ErrImagePull|ImagePullBackOff"}` (kube-state-metrics) to see it sooner.
- Privileged Docker-in-Docker: a job that escapes its container gets root on the node. Run it only for repositories you control, with the NetworkPolicy above.
- Polling only (Forgejo has no job webhooks yet), so a job waits up to `POLL_INTERVAL` plus the pod start time. Every pod pulls the runner, dind and job images again unless the node has them cached.
- The token used for polling has the scope of a Forgejo personal access token (coarse). Use a bot user that only belongs to the served organisation.
- Tested against Forgejo 15.0.11 and runner 13.2.1: a real job was picked up and run with `one-job --handle` (host-mode label). The pod itself, with dind, has not run in a cluster yet; the manifests were validated by the Kubernetes API server in dry-run mode.
