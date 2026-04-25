# homelab

Kubernetes home lab with Talos OS, FluxCD, and custom applications.

## Quick Start

1. Deploy cluster: See [cluster/magi/talos/](cluster/magi/talos/README.md)
2. Bootstrap FluxCD: `flux bootstrap github --owner=... --repository=homelab`
3. Deploy apps: `kubectl apply -k overlays/magi/databases/`

## Structure

- `apps/` - Base app definitions
- `cluster/magi/` - Cluster config + FluxCD
- `overlays/magi/` - Per-cluster overlays
- `containers/` - Custom Docker images

## Security

No plaintext secrets in git. All secrets encrypted with SOPS + age.
