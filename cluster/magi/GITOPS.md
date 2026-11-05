# MAGI GitOps (Flux + SOPS)

Flux syncs `cluster/magi/` from `git@github.com:vbalexr/homelab.git` (branch `main`). Secrets are age-encrypted
with SOPS and decrypted by Flux in the cluster. Cilium and the Talos config are not managed by Flux; they are
installed by hand (see `talos/README.md`).

## Layout

| Path | Content |
|---|---|
| `cluster/magi/kustomization.yaml` | Root of the sync path: `flux-system/` and one file per layer |
| `cluster/magi/flux-system/` | Written by `flux bootstrap` (controllers and the sync definition). Do not edit |
| `cluster/magi/infrastructure.yaml` | Flux `Kustomization` for `overlays/magi/infrastructure` (cluster services, CRDs, storage) |
| `cluster/magi/configurations.yaml` | `overlays/magi/configurations` (cert-manager issuers); `dependsOn: infrastructure` |
| `cluster/magi/databases.yaml` | `overlays/magi/database` (namespace `database`: postgres, mariadb); `dependsOn: configurations` |
| `cluster/magi/<group>.yaml` | One application group each (`shared`, ...), `dependsOn: databases` |
| `overlays/magi/<layer or group>/` | What each deploys. A group has its own namespace, Gateway (fixed LB address), Certificate, routes and one folder per app, listed in its `kustomization.yaml` |
| `.sops.yaml` | Encrypts `data`/`stringData` of any YAML under `overlays/magi/` for the cluster age key |
| `.hooks/check-sops-encrypted.py`, `.gitleaks.toml`, `.pre-commit-config.yaml` | Block commits of plaintext Secrets or key material |

Every layer uses `decryption.provider: sops` with the `sops-age` secret in `flux-system`. A new layer is a new
`cluster/magi/<name>.yaml` plus an entry in `cluster/magi/kustomization.yaml`.

## Set up (once)

Tools: `sudo pacman -S sops age pre-commit`, and the `flux` CLI.

```sh
# 1. age key for the cluster. Back it up to /mnt/ShareDrive/Secrets/; it is git-ignored and unrecoverable.
age-keygen -o cluster/magi/.sops.age && chmod 600 cluster/magi/.sops.age
grep '^# public key:' cluster/magi/.sops.age        # put this value in .sops.yaml as the `age:` recipient

# 2. namespace and decryption secret, before Flux needs them
export KUBECONFIG=$PWD/cluster/magi/talos/generated/kubeconfig
kubectl create namespace flux-system
kubectl -n flux-system create secret generic sops-age --from-file=age.agekey=cluster/magi/.sops.age

# 3. hooks
pre-commit install

# 4. bootstrap (needs the repo pushed to main and GITHUB_TOKEN with repo scope; adds a deploy key)
flux check --pre
flux bootstrap github --owner=vbalexr --repository=homelab --branch=main --path=cluster/magi --personal
flux get kustomizations
```

## Day-2

```sh
# new or edited secret: write it as a normal Secret manifest under overlays/magi/, then encrypt in place
sops --encrypt --in-place overlays/magi/apps/<app>/secret.yaml
sops overlays/magi/apps/<app>/secret.yaml           # edit an encrypted file (decrypts in $EDITOR)

# reconcile now instead of waiting for the interval
flux reconcile kustomization flux-system --with-source
flux get kustomizations; flux get helmreleases -A
```

- Recipients live in `.sops.yaml`. To rotate the key, change the recipient and run `sops updatekeys <file>` on every
  encrypted file, then replace the `sops-age` secret.
- `encrypted_regex: ^(data|stringData)$` keeps names and metadata readable in Git; only values are encrypted.

## Image automation

The Flux image-reflector and image-automation controllers (namespace `flux-system`) keep these images current:
postgres, aa-sspti, aa-sspti-nginx, chilu-bb, cloudflared, immich-server, immich-machine-learning, vaultwarden.
All other images stay pinned by hand.

- Each group folder (`database`, `sspti`, `chilu`, `shared`, `tunnels`) has an `image-automation.yaml` with an `ImageRepository`
  and `ImagePolicy` per image and an `image-update.yaml` with an `ImageUpdateAutomation` per app, all in `flux-system`. Private registries use the
  `ghcr-pull-*` secret in `secret-image-pull.yaml` of the same folder.
- The workload manifests use the bare repository name. The tag lives in the app's `kustomization.yaml` under `images:` as
  `newTag: <tag> # {"$imagepolicy":"flux-system:<policy>:tag"}`. The automation rewrites that value and pushes
  `chore(images): update <app> ...` to `main` as `fluxcdbot` (the `flux-system` deploy key is read-write).
- Policies: postgres, aa-sspti, aa-sspti-nginx and chilu-bb take the newest `YYYYMMDD-HHMM` (postgres `YYYYMMDD.HHMM`) tag;
  immich (server and machine-learning move together) follows semver `>=3.0.0 <4.0.0`; vaultwarden `>=1.36.0 <2.0.0`;
  cloudflared `>=2026.5.0`.
- The `check-postgres-updates` GitHub workflow (Mondays 02:00 UTC) builds new postgres tags; the policy then rolls them out automatically.
- `image-update.yaml` is listed in the group `kustomization.yaml` only while its automation is active. A `suspend: true`
  automation committed to Git never reports Ready, so it would stall the group's Flux Kustomization (`wait: true`) and every group that depends on it.
- Pause: `flux suspend image update <name>` (resume with `flux resume image update <name>`); for a lasting pause remove
  `image-update.yaml` from the group `kustomization.yaml`. Inspect with `flux get image repository`, `flux get image policy`, `flux get image update`.
