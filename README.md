# homelab

Kubernetes home lab with Talos OS, FluxCD, and custom applications.

## Structure

- `apps/` - Base app definitions
- `cluster/magi/` - Cluster config + FluxCD
- `overlays/magi/` - Per-cluster overlays
- `containers/` - Custom Docker images

## Security

No plaintext secrets in git. All secrets encrypted with SOPS + age.
