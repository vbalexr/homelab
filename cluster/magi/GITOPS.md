# MAGI GitOps (Flux + SOPS)

Flux syncs `cluster/magi/` from `git@github.com:vbalexr/homelab.git` (branch `main`). Secrets are age-encrypted
with SOPS and decrypted by Flux in the cluster. Cilium and the Talos config are not managed by Flux; they are
installed by hand (see `talos/README.md`).

## Layout

| Path | Content |
|---|---|
| `cluster/magi/kustomization.yaml` | Root of the sync path: `flux-system/`, `infrastructure.yaml`, `apps.yaml` |
| `cluster/magi/flux-system/` | Written by `flux bootstrap` (controllers and the sync definition). Do not edit |
| `cluster/magi/infrastructure.yaml` | Flux `Kustomization` for `overlays/magi/infrastructure` (cluster services, CRDs, storage) |
| `cluster/magi/apps.yaml` | Flux `Kustomization` for `overlays/magi/apps`; `dependsOn: infrastructure` |
| `overlays/magi/<layer>/` | What each layer deploys. Add one folder per component and list it in the layer's `kustomization.yaml` |
| `.sops.yaml` | Encrypts `data`/`stringData` of any YAML under `overlays/magi/` for the cluster age key |
| `.hooks/check-sops-encrypted.py`, `.gitleaks.toml`, `.pre-commit-config.yaml` | Block commits of plaintext Secrets or key material |

Both layers use `decryption.provider: sops` with the `sops-age` secret in `flux-system`. A new layer is a new
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
