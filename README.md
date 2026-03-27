# homelab

Personal home lab repository for network configuration and Kubernetes cluster management with FluxCD.

## Repository Structure

```
homelab/
├── apps/                              # Application definitions (base Kustomize resources)
│   └── <app-name>/                    # One directory per application
│
├── cluster/                           # Cluster-specific configurations
│   └── <cluster-name>/                # Base FluxCD bootstrap + server configurations
│       ├── flux-system/               # FluxCD system manifests (auto-generated)
│       └── infrastructure/            # Cluster-level infrastructure configs
│
├── overlays/                          # Application overlays per cluster/namespace
│   └── <cluster-name>/
│       └── <namespace>/               # Kustomize overlays for a specific cluster + namespace
│
├── network/                           # Network documentation and configuration
│   └── README.md                      # Network topology, VLANs, firewall rules, etc.
│
└── containers/                        # Custom-built container images
    └── <app-name>/                    # Dockerfile + build context for each custom image
```

## Directory Descriptions

| Directory | Purpose |
|-----------|---------|
| `apps/` | Base application definitions (Deployments, Services, ConfigMaps, etc.) shared across clusters. |
| `cluster/<cluster-name>/` | Per-cluster FluxCD bootstrap manifests and server-level configurations. |
| `overlays/<cluster>/<namespace>/` | Kustomize overlays that customise base apps for a specific cluster and namespace. |
| `network/` | Network documentation: topology diagrams, VLAN assignments, firewall rules, DNS records. |
| `containers/` | Dockerfiles and build contexts for applications that require custom container images. |

## Security

This repository enforces the following security controls:

- **No secrets in plaintext** — All secrets must be encrypted (e.g. with [SOPS](https://github.com/mozilla/sops) + age/GPG) before committing.
- **No private keys** — SSH private keys, RSA keys, PEM files, and similar credentials must never be committed.
- **Automated secret scanning** — A GitHub Actions workflow runs [Gitleaks](https://github.com/gitleaks/gitleaks) on every push and pull request to detect accidentally committed secrets.
- **Pre-commit hooks** — Install the provided pre-commit configuration to catch secrets locally before they reach the remote.

### Setting Up Pre-commit Hooks

```bash
# Install pre-commit
pip install pre-commit

# Install the hooks defined in .pre-commit-config.yaml
pre-commit install
```

### Encrypting Secrets with SOPS

```bash
# Encrypt a Kubernetes secret manifest
sops --encrypt --in-place secrets.yaml
```

Never commit unencrypted `Secret` manifests. Use [sealed-secrets](https://github.com/bitnami-labs/sealed-secrets) or SOPS as an alternative.

## Getting Started

1. Fork / clone this repository.
2. Set up your cluster directory under `cluster/<your-cluster-name>/`.
3. Bootstrap FluxCD pointing at that directory.
4. Add application base manifests under `apps/`.
5. Create overlays under `overlays/<your-cluster>/<namespace>/` to customise per environment.
