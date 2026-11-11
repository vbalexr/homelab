# forgejo-runners

The `forgejo-job-controller` (image from `containers/forgejo-job-controller`) and, in the same namespace, the pods it starts: one per Forgejo Actions job.
Namespace `forgejo-runners` is `privileged` because each job pod has a Docker-in-Docker sidecar.

**Not wired into Flux yet.** `cluster/magi/` has no entry for the `forgejo` group. Before adding it:

1. Forgejo is running, and an organisation exists whose jobs these runners serve (`FORGEJO_SCOPE` in `configmap.yaml`).
2. A bot user in that organisation has a personal access token that can manage runners. Store it and uncomment `secret.yaml` in `kustomization.yaml`:

   ```sh
   kubectl create secret generic forgejo-controller-token -n forgejo-runners --from-literal=token=<pat> --dry-run=client -o yaml \
     > overlays/magi/forgejo/runners/secret.yaml
   sops --encrypt --in-place overlays/magi/forgejo/runners/secret.yaml
   ```
3. A NetworkPolicy that keeps job pods (`app.kubernetes.io/name: forgejo-job`) away from the cluster's other services is written and tested.

The image tag is kept current by Flux image automation (`../image-automation.yaml`, `../image-update.yaml`): the newest `YYYYMMDD-HHMM` tag.
Runner and dind images are pinned by hand in `configmap.yaml`.
