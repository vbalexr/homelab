# Forgejo + runners: investigation notes (TEMPORARY)

Scratch notes from the research phase, written 2026-11-11. Delete this file once the real `README.md` for the app exists.
Nothing here is deployed. "Verify" marks things I could not confirm from the docs and must check at implementation time.

## 0. Decided (2026-11-11)

- Host `git.vbalex.com`, web and SSH on one Gateway IP (`10.1.1.17`), SSH through a TCP listener and a `TCPRoute` (v1; the chart's own template uses v1alpha2, which the cluster CRDs no longer serve).
- DNS is automatic: external-dns publishes the HTTPRoute hostname. Reachable only from networks the routers/firewall allow to the service IPs (no tunnel, no Tailscale).
- Forgejo 15.0.11 LTS from chart 17.3.2, rootless, 100Gi `longhorn-retain` PVC `forgejo-data`, database `forgejo` (role `forgejo`) on the cluster postgres.
- Runners: one pod per job through `containers/forgejo-job-controller`, not wired until `runners/README.md` is done.
- Written in `forgejo/` and the group files; `cluster/magi/forgejo.yaml` exists but is not listed in `cluster/magi/kustomization.yaml` yet.

## 1. Decisions to take (recommendations first)

| # | Question | Recommendation | Why |
|---|---|---|---|
| 1 | Where does Forgejo run | In the cluster, new group `forgejo` | Flux syncs from GitHub, not Forgejo, so there is no bootstrap loop. Longhorn already backs up to the NAS |
| 2 | Source of truth for this repo | Keep GitHub. Add Forgejo as a mirror later | The image-automation controllers push to GitHub, and Flux needs a repo that stays up when the cluster is down |
| 3 | Version | Forgejo **15 (LTS)** | Docs list v17.0.1 as latest and v15.0.11 as LTS (stable every 3 months, LTS yearly). The chart v17 deploys Forgejo v15 by default |
| 4 | Install method | Helm chart `oci://code.forgejo.org/forgejo-helm/forgejo` as a Flux `HelmRelease` | Chart handles `app.ini`, secrets, SSH service, metrics. The repo already uses HelmReleases. Alternative is plain manifests like vaultwarden, more control but we own `app.ini` |
| 5 | Database | Existing cluster postgres (18.6), new role and database | Forgejo needs PostgreSQL 14 or newer. SQLite would work for a homelab but the postgres is already backed up and in use by other apps |
| 6 | Runners | **Pod per job** with the experimental `runner-k8s-plugin` (section 5.1), in a separate namespace. DinD (section 5.2) is the fallback | You asked for a pod per job. No privileged containers, jobs are scheduled by Kubernetes. Cost: experimental, third-party code, image builds need Buildah or BuildKit |
| 7 | Who can reach it | **Open**: LAN only, Tailscale, or Cloudflare tunnel (all three exist in the repo) | Forgejo's own security guide says to restrict instance access, because runners execute code |

## 2. What the repo already gives us (conventions to follow)

- **Group layout:** `cluster/magi/<group>.yaml` (Flux Kustomization, `dependsOn: databases`, SOPS decryption) plus `overlays/magi/<group>/` with
  `namespace.yaml`, `gateway.yaml`, `certificate.yaml`, one folder per app, and a `kustomization.yaml`. New groups are listed in `cluster/magi/kustomization.yaml`.
- **Closest template:** `overlays/magi/shared/vaultwarden/` (Deployment + `Recreate` + PVC on `longhorn-retain` + Service + HTTPRoute + HTTPRoute redirect + SOPS secret + README with restore notes).
- **HTTP exposure:** a Cilium Gateway per group with a fixed LB IP from the pool (`lbipam.cilium.io/ips`, dual-stack `10.1.1.x` and `fd7a:2201:1ac::x`), TLS from a cert-manager `Certificate` (`letsencrypt-prod`, DNS-01 through Cloudflare),
  HTTPRoute for the host plus a 301 redirect route on the `http` listener.
- **DNS:** external-dns creates the Technitium record from each HTTPRoute hostname (source is only `gateway-httproute`).
- **Postgres:** `postgres.database.svc.cluster.local:5432`. Naming used so far is `<namespace>_<app>` for role and database (`shared_vaultwarden`, `shared_immich`). Locale is `en_US.utf8`.
  Role and database are created by hand (see the restore steps in `shared/vaultwarden/README.md`), credentials go in a SOPS secret.
- **Pod security:** namespaces are `baseline`. Only `longhorn-system` and `monitoring` are `privileged`. No NetworkPolicies exist outside flux-system and longhorn-system.
- **Secrets:** SOPS age, `stringData` encrypted in place (`sops --encrypt --in-place`). Pre-commit hooks block plaintext.
- **Backups:** the Longhorn `daily-backup` job covers every volume in the default group (03:00, keep 15, NFS target on the NAS). New PVCs are covered automatically.
- **IPs in use:** `.10`-`.16` gateways/minecraft, `.20` postgres, `.21` mariadb, `.23` mosquitto, `.24` truenas exporter, `.30`/`.31` technitium.
- **Capacity:** 3 nodes, 16 Atom cores and 32 GB each. Memory is already at 32-53 %, CPU at 9-16 %. The Atom cores are slow, so CI builds will be slow.
- **Observability rule from CLAUDE.md:** Prometheus metrics by default. Forgejo exposes them (`[metrics] ENABLED = true`, chart has a ServiceMonitor option; verify). Forgejo logs are not JSON with trace IDs, so that rule cannot be met for this app.

## 3. Forgejo server

**Chart facts (from the chart README):**
- `image.rootless: true` by default, image from `code.forgejo.org/forgejo/forgejo`.
- `strategy.type: Recreate` (docs warn against RollingUpdate; the RWO PVC would deadlock, same problem Grafana just had). `replicaCount: 1`, not HA.
- Persistence on by default, 10Gi RWO, supports `claimName` for an existing PVC.
- Since chart v14 the PostgreSQL and Valkey subcharts are gone, so an external DB is required. Config goes under `gitea.config.database` (`DB_TYPE: postgres`, `HOST`, `NAME`, `USER`, `PASSWD`) or env vars `FORGEJO__DATABASE__*` through `gitea.additionalConfigFromEnvs`. The README warns that blind upgrades across older versions could delete a bundled Postgres, so back up before any chart upgrade.
- Admin user: `gitea.admin` (`username`, `passwordMode: keepUpdated` default, or `existingSecret`). Use `existingSecret` from SOPS.
- Secrets that must stay stable across reinstalls: `SECRET_KEY` and the internal token. If the chart regenerates them on a reinstall, encrypted values in the DB (2FA, tokens) become unreadable. Pin them in a SOPS secret (verify how the chart exposes this).
- SSH: `service.ssh` defaults to ClusterIP port 22. SSH through an ingress or Gateway is **not supported**, so it needs a LoadBalancer (options: `loadBalancerIP`, `annotations`, `externalTrafficPolicy`; verify the exact keys).

**Database setup (run on the cluster postgres):**
```sql
CREATE ROLE forgejo_forgejo WITH LOGIN PASSWORD '<from SOPS>';
CREATE DATABASE forgejo_forgejo WITH OWNER forgejo_forgejo TEMPLATE template0
  ENCODING UTF8 LC_COLLATE 'en_US.utf8' LC_CTYPE 'en_US.utf8';
```
No TLS is needed inside the cluster per the Forgejo docs (private network). There is no `pg_hba` customisation in this repo, so the defaults apply (verify the pod CIDR is allowed, as it is for the other apps).

**Sizing (decided):** requests 100m CPU and 256Mi, memory limit 1Gi. PVC **100Gi** on `longhorn-retain` (volume expansion is allowed). Repos, LFS, attachments and Actions logs live on it, and a container registry or packages would grow it quickly. At 2 replicas that is 200Gi of Longhorn disk across the nodes.

**Caches and queues:** leave the defaults (memory cache, leveldb queue on disk, bleve indexer). Valkey exists in the cluster but is not needed for one instance.

## 4. Exposure

- **HTTP:** HTTPRoute on a new group Gateway. Proposed host `git.vbalex.com` (needs adding to the group `Certificate`, external-dns then creates the record). New Gateway IP: `10.1.1.17` / `fd7a:2201:1ac::17`.
- **SSH on the same IP (decided: try option A):** a `TCP` listener on port 22 of the group Gateway plus a `TCPRoute` to the chart's ClusterIP SSH Service. Web and SSH share `10.1.1.17`, so one hostname (`git.vbalex.com`) serves both, `SSH_DOMAIN` equals the web domain, and external-dns needs no change.
  - The `tcproutes` CRD is already installed, and Cilium 1.20.4 lists TCPRoute as supported (CRD must exist or Cilium disables it).
  - Listener: `name: ssh, protocol: TCP, port: 22, allowedRoutes: {kinds: [{kind: TCPRoute}], namespaces: {from: Same}}`.
  - Drawbacks to test: SSH goes through Envoy so Forgejo sees Envoy's address, and Envoy idle or stream timeouts could cut a very large push or clone. Test a large clone and a long idle session before relying on it.
  - Fallback if it misbehaves: a separate LoadBalancer Service with its own IP (proposed `10.1.1.22`) and its own hostname. That needs a hand-made Technitium record, because external-dns only watches `gateway-httproute`. Sharing an IP between the Gateway Service and another Service with `lbipam.cilium.io/sharing-key` is not an option, since the Gateway Service has no selector and combining sharing with BGP can drop traffic.
  - The routers only accept /32 and /128 routes inside the LB pool, and a new port on an existing IP needs no router change.
- **Hardening from the Forgejo security guide:** disable open registration, disable forking if not needed, require 2FA, limit repository creation, keep the instance off the open internet unless needed.

## 5. Runners

**What a runner is:** Forgejo does not run jobs. `forgejo-runner` (image `data.forgejo.org/forgejo/runner:<major>`, examples show 13.x; verify the latest) polls Forgejo and runs workflows from `.forgejo/workflows`. The docs state plainly that it performs remote code execution.

**Is there a Kubernetes-native runner?** Not officially. `forgejo-runner` only runs jobs as Docker/Podman containers, LXC or on the host. Upstream tracks it in [discussion #66](https://codeberg.org/forgejo/discussions/issues/66) and is looking at a plugin interface for backends. Community projects fill the gap:
- `sjones512/forgejo-runner-kubernetes` and `forge-runner-k8s`: one pod per job, no DinD, but experimental.
- `ultimaterex/forgejo-runner-k8s` (KEDA), `neodymium6/forgejo-ephemeral-runner`: ephemeral runner pods that still use DinD inside.
- Homelab repos using a "k8s plugin" with `ubuntu-latest:k8s:///...` labels exist, but I could not verify they are stable.
Update: see 5.1. The plugin architecture exists since runner 13.1 and is experimental. DinD stays the fully supported path.

**Options for running containers inside jobs (older research, before the plugin finding):**

| Option | Fit for Talos/Kubernetes |
|---|---|
| Docker-in-Docker (`docker:dind` privileged sidecar) | **Yes.** Official example `examples/kubernetes/dind-docker.yaml`. Isolated from the node's own container runtime, but the sidecar is `privileged: true` |
| Socket automount | No. Talos has no Docker socket and it gives no isolation anyway |
| LXC labels | No. Needs LXC on the host, not available on Talos |
| Runner in a VM | Not available here (no hypervisor in the cluster) |
| Rootless BuildKit StatefulSets (homelab write-ups) | Possible later for image builds only. More moving parts, not an official path |

### 5.1 Pod per job: `runner-k8s-plugin` (chosen direction)

**Status found (2026-11-11):** the official runner (v13.1.0 and later) now has a **plugin architecture**, still labelled *experimental* by the Forgejo maintainers (Gonza-man, 2026-10-09: it stays experimental until there is wider adoption; feedback goes to `forgejo/runner` issue 1773). Backends live outside the runner as a plugin process that the runner talks to over a Unix socket. Background: [discussion #66](https://codeberg.org/forgejo/discussions/issues/66). The Kubernetes backend was first proposed upstream (runner PR 1472), and the maintainers asked for the plugin interface first.

**The plugin:** [eleboucher/runner-k8s-plugin](https://git.erwanleboucher.dev/eleboucher/runner-k8s-plugin) (GPL-3.0, same as the runner; mirrors on code.forgejo.org and GitHub; has a Helm chart `charts/forgejo-runner`).
- Works with the **official runner image** (`data.forgejo.org/forgejo/runner:13.x`), no patched fork needed. The plugin runs as a native sidecar next to the runner.
- **One pod per job**, with a shared volume and a custom PodSpec per label (`ubuntu-latest:k8s:///config/podspec-default.yaml`). A container named `main` executes the steps (added automatically if missing). Service containers (`services:`) become one extra Job and pod each, reachable by name through `hostAliases`. They do not mount the shared workspace.
- Backend options: `namespace`, `kubeconfig` (in-cluster default), `poll_timeout` (10m), `podspec`, `image_pull_policy`, `labels` (applied to job pods), `resources`.
- Runner config sets `plugins.k8s.address: unix:///plugin/forgejo-runner-k8s.sock`. Runner and plugin share an `emptyDir` at `/plugin`. The runner reads UUID and token from files in a mounted secret.
- **RBAC** (Role in the runner namespace only), as used by two homelab deployments: `batch/jobs` create/get/list/watch/delete, `pods` get/list/watch, `pods/exec` create/get, `pods/log` get/list/watch. `automountServiceAccountToken` must stay on for the runner pod (one user broke it by disabling it). Job pods themselves should get `automountServiceAccountToken: false` in the PodSpec.
- Two homelab write-ups report it working: [jw2dotorg/iac #667](https://github.com/jw2dotorg/iac/pull/667) and [davealtena/homelab #2363](https://github.com/davealtena/homelab/pull/2363) (capacity 2-3, job timeout 1-8h, job pod requests 100m/256Mi, memory limit 2Gi, workdir and cache on `emptyDir`).

**Risks and gaps to accept or fix:**
- **Experimental**, one maintainer, and other users say they would not rely on it in production yet. For a homelab CI that is acceptable. Keep DinD as the fallback.
- **Third-party images.** The plugin image is published on the author's own registry (`registry.erwanleboucher.dev`, also packages on `code.forgejo.org/eleboucher`). It runs with permission to create Jobs in the cluster. Better to build it from source ourselves (a Dockerfile exists, Go) in our GitHub Actions and push to ghcr, then pin by digest. Read the source and chart before first use.
- **Image builds without Docker.** No DinD means `docker build` does not work in steps. Use a rootless **Buildah** or **BuildKit** step in a custom PodSpec/label (a user shared a Buildah configuration in the discussion; not verified here). This is where our current workflows (postgres, aa-sspti to ghcr) need rework. Rootless builds still need relaxed seccomp/user-namespace settings on a `baseline` namespace; check what PodSecurity allows.
- **Caching:** no persistent Docker layer cache (workdir and cache are `emptyDir` in the examples). Layer caching for builds needs a registry cache or a BuildKit cache.
- **Version drift:** examples pair runner 13.2.0 with plugin 13.2.1. Pin both and upgrade together. Check the plugin against the Forgejo server version we pick.
- **Actions that expect Docker** (`docker/build-push-action`, `docker run` steps, `container:` jobs with service sockets) will not work as-is.
- Job pods are not privileged, but they still run arbitrary code with cluster network access. The `CiliumNetworkPolicy` in the security notes below applies to the job namespace and is more important here, because job pods are scheduled on any node.

**Layout for this option (replaces the DinD files in section 6):** `forgejo-runners` namespace (`baseline`, no privileged), `ServiceAccount` + `Role` + `RoleBinding`, `ConfigMap` (runner config and podspecs), `Secret` (UUID and token), `Deployment` of the runner with the plugin as native sidecar, `CiliumNetworkPolicy`. Job pods run in the same namespace.

### 5.2 Fallback: Docker-in-Docker (supported, but not pod per job)

**DinD design (native sidecar):**
- Deployment (or StatefulSet) in a **separate namespace** `forgejo-runners` with `pod-security.kubernetes.io/enforce: privileged`, same pattern as `monitoring`. The Forgejo namespace stays `baseline`.
- DinD as an init container with `restartPolicy: Always` (native sidecar, Kubernetes 1.29+, the cluster is on 1.37). A startup probe runs `docker info`, so the runner only starts once the daemon is up and stops first on shutdown.
- **Use a Unix socket, not TCP.** The compose and official examples expose `dockerd` on `tcp://...:2375` or `:2376`. In the cluster that is an unauthenticated root-equivalent API for any pod. Use `--host=unix:///run/dind/docker.sock --group=1000` on a shared `emptyDir`, and make `dockerd` the first argument so the image entrypoint does not add a TCP listener.
- Runner container: non-root (uid 1000), read-only root filesystem, no service account token, `DOCKER_HOST=unix:///run/dind/docker.sock`.
- `container.privileged` stays `false` in the runner config, and `container.network` is not `host`.
- Resource limits on both containers (the official example uses 1 CPU and 4Gi each; with 16 slow Atom cores shared with everything else, start lower and set `runner.capacity: 1`).
- Docker storage is an `emptyDir` with a `sizeLimit`. Images are pulled again after every pod restart (one write-up measured 13 minutes for the first pull of the runner image alone). Acceptable for a start. A pull-through registry mirror can come later.
- Set an MTU on the DinD daemon if the pod network is an overlay (a KEDA variant recommends 1400; check the Cilium MTU here).
- Labels in `config.yml`, for example `ubuntu-latest:docker://node:lts` style mappings, pinned by digest as the docs recommend. `server.connections` must not be in the config file when connection details come from flags, or the runner refuses to start.

**Registration, two ways:**
1. **Token in the UI** (docs say recommended): create the runner in the Forgejo UI, copy the UUID and token. One manual step, then both go into a SOPS secret.
2. **Offline registration with a shared secret**, meant for IaC: generate a 40-character hex secret, put it in SOPS, then run once inside the Forgejo pod:
   `forgejo forgejo-cli actions register --name <name> --scope <org> --secret <hex>`. It prints the UUID. The runner then uses UUID and secret as its token.
   Registration lives in the Forgejo database, so it has to be repeated after a restore from scratch.
- Pass the token as a file (`--token-url=file:...`) so it is not in the process list.
- Pick the narrowest scope: repository, then organisation or user. Avoid instance-wide runners.
- **Ephemeral mode** (`--ephemeral`, one job per runner) is the strongest isolation and is enforced server side. It needs something that re-registers a runner per job (the KEDA `ScaledJob` variant). Start persistent with a single replica and revisit.

**Security notes that matter for this cluster:**
- A job that escapes its container gets root on the node because DinD is privileged. Only run workflows from repositories we control, keep registration closed, and do not give jobs registry credentials they do not need.
- Add a `CiliumNetworkPolicy` for `forgejo-runners`: allow DNS, Forgejo (HTTP) and the internet, deny the rest of the cluster (postgres, mariadb, Longhorn, the Kubernetes API, admin consoles). The cluster has no policies today, so this is new work and worth doing before runners go live.
- Lower `runner.timeout` from the 3 hour default. Pin third-party actions to commit SHAs.
- `actions/checkout` clones from the instance `ROOT_URL`, so `git.vbalex.com` must resolve and be reachable from pods (Technitium via external-dns, and the LB IP over BGP). The runner itself can talk to the in-cluster Service URL.

**Workflow compatibility:** Forgejo Actions reads `.forgejo/workflows`. The repo's current workflows are in `.github/workflows` and push to ghcr (`build-postgres-image`, `build-aa-sspti-images`, `check-postgres-updates`). Moving them is a separate step. The Flux image automation depends on ghcr tags, so keep ghcr as the registry for now. Forgejo has its own container registry, which could replace it later.

## 6. Proposed layout

```
cluster/magi/forgejo.yaml                 Flux Kustomization, dependsOn: databases
overlays/magi/forgejo/
  NOTES.md                                this file (temporary)
  kustomization.yaml
  namespace.yaml                          forgejo, baseline
  gateway.yaml                            10.1.1.17 / fd7a:2201:1ac::17
  certificate.yaml                        git.vbalex.com
  forgejo/
    helmrepository.yaml                   type: oci, code.forgejo.org
    helmrelease.yaml                      rootless, Recreate, external postgres, metrics
    secret.yaml                           SOPS: admin, DB password, secret key, runner secret
    httproute.yaml                        https + http redirect
    tcproute-ssh.yaml                     TCPRoute: Gateway listener 22 -> chart SSH Service
    README.md
  runners/
    namespace.yaml                        forgejo-runners, privileged
    configmap.yaml                        runner config (labels)
    secret.yaml                           SOPS: UUID + token
    deployment.yaml                       runner + DinD native sidecar
    networkpolicy.yaml                    CiliumNetworkPolicy
    README.md
```

Open point: the runners could live in the same Flux group (one Kustomization, simpler) or in their own (`forgejo-runners.yaml`, so a stuck runner cannot block Forgejo through `wait: true`). I lean to one group, two namespaces.

## 7. Build order

1. Create role and database on postgres by hand.
2. Group scaffolding, namespace, Gateway, Certificate, SOPS secrets. Check the staging issuer first if the hostname is new.
3. `HelmRelease` for Forgejo. Log in as admin, create the user and organisation, turn registration off.
4. SSH: TCP listener and TCPRoute. Test `git clone`, a large clone and an idle session.
5. Metrics: ServiceMonitor, then a Grafana dashboard if wanted.
6. Runners namespace and network policy first, then register a runner and run a test workflow (`runs-on: ubuntu-latest`, then a `docker build`).
7. Optional: pull mirror of the GitHub repo, migrate workflows, `forgejo dump` CronJob to the NAS.

## 8. Backup and restore

- Volume: covered by the Longhorn daily backup to the NAS.
- Database: lives in the postgres volume, also backed up, but the two backups are taken independently, so they are not atomic. Repos on disk and the DB can drift by up to a day. A `forgejo dump` CronJob (or a `pg_dump` plus a repo snapshot taken together) gives a consistent copy.
- Restore: same shape as the vaultwarden README. Scale to 0, restore DB and `/data`, scale up, re-register runners.

## 9. Open questions for the user

1. Reachability: LAN only, Tailscale, or public through the Cloudflare tunnel?
2. Hostname: `git.vbalex.com` for both (SSH shares the Gateway IP).
3. Forgejo 15 LTS or 17 (latest)?
4. Chart or plain manifests?
5. Runner scope (org or user) and how many parallel jobs the Atoms should take.
6a. Are you fine with an experimental, third-party plugin that we build ourselves, and with reworking the image-build workflows to Buildah/BuildKit?
6. Move the GitHub workflows to Forgejo Actions now or later?

## Sources

- Forgejo runner overview: https://forgejo.org/docs/latest/admin/actions/runner-installation/
- Runner on Docker: https://forgejo.org/docs/latest/admin/actions/installation/docker/
- Registration: https://forgejo.org/docs/latest/admin/actions/registration/
- Configuration and labels: https://forgejo.org/docs/latest/admin/actions/configuration/
- Docker access in workflows: https://forgejo.org/docs/latest/admin/actions/docker-access/
- Securing Actions: https://forgejo.org/docs/latest/admin/actions/security/
- Installation and versions: https://forgejo.org/docs/latest/admin/installation/
- PostgreSQL preparation: https://forgejo.org/docs/latest/admin/installation/database-preparation/
- Helm chart: https://code.forgejo.org/forgejo-helm/forgejo-helm
- Official k8s runner example: https://code.forgejo.org/forgejo/runner/raw/branch/main/examples/kubernetes/dind-docker.yaml
- DinD native sidecar write-up: https://jddemonteverde.com/blog/k3s-homelab/forgejo-runner
- Other write-ups: https://init6.sh/posts/2025/10/deploying-forgejo-runners-on-rke2/ , https://github.com/ultimaterex/forgejo-runner-k8s , https://www.flexguse.de/self-hosted-forgejo-kubernetes-runner/
