# forgejo-job-controller

Starts one Kubernetes pod per waiting Forgejo Actions job. Go, standard library only.

## Configuration (environment)

| Variable | Default | Meaning |
|---|---|---|
| `FORGEJO_URL` | required | Forgejo base URL |
| `FORGEJO_RUNNER_URL` | `FORGEJO_URL` | URL the job pods use to reach Forgejo |
| `FORGEJO_SCOPE` | required | `orgs/<org>`, `repos/<owner>/<repo>`, `user` or `admin` |
| `FORGEJO_TOKEN_FILE` / `FORGEJO_TOKEN` | required | Forgejo access token |
| `LABELS` | required | `runs-on` label to image: `name=docker://image,...` |
| `RUNNER_IMAGE`, `DIND_IMAGE` | required | Pinned images for the job pods |
| `RUNNER_CONFIGMAP` | none | ConfigMap with the runner `config.yml` |
| `NAMESPACE` | service account namespace | Namespace of the job pods |
| `POLL_INTERVAL` | `5s` | |
| `MAX_JOBS` | `2` | Parallel job pods |
| `JOB_TIMEOUT` | `2h` | Pod deadline |
| `LONG_RUNNING_AFTER` | `45m` | Warn about job pods older than this |
| `FINISHED_TTL` | `30m` | How long finished Jobs are kept |
| `JOB_CPU_LIMIT`, `JOB_MEMORY_LIMIT`, `JOB_DISK_LIMIT` | `2`, `4Gi`, `10Gi` | Default job pod limits |
| `PROFILES` | none | Resource classes by `runs-on` label: `name:cpu=8,memory=12Gi,disk=30Gi,max=2;...` (a name in `LABELS` also picks an image; a name only in `PROFILES` is a size label that takes the image of the job's other label, e.g. `runs-on: [ubuntu-latest, large]`) |
| `DEFAULT_LABEL` | none | Image label used by jobs that only name a size label, so `runs-on: large` works alone |
| `PRIORITY_CLASS` | none | PriorityClass of the job pods |
| `DIND_MTU` | unset | MTU of the job pod Docker daemon |
| `LISTEN` | `:8080` | `/metrics` and `/healthz` |
| `LOG_LEVEL` | `info` | |
| `KUBE_API_URL`, `KUBE_TOKEN_FILE`, `KUBE_DRY_RUN` | in cluster | Local testing against `kubectl proxy` |

`-once` runs one poll cycle and exits.

## Observability

JSON logs with `trace_id` on every line. Prometheus metrics on `/metrics` (`fjc_*`).

## Develop

```bash
go vet ./... && go test ./...
docker build -t ghcr.io/vbalexr/forgejo-job-controller:dev .
```

CI (`.github/workflows/build-forgejo-job-controller.yml`) tests the code and pushes the image to ghcr.
