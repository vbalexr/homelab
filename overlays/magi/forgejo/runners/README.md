# forgejo-runners

`forgejo-job-controller` (image built from `containers/forgejo-job-controller`) and the job pods it starts for Forgejo Actions.

Not enabled yet: `runners` is commented out in `../kustomization.yaml`. It needs the `forgejo-controller-token` secret (key `token`),
encrypted with SOPS as `secret.yaml` and listed in `kustomization.yaml`.

The controller image tag is updated by Flux image automation (`../image-automation.yaml`, `../image-update.yaml`).
