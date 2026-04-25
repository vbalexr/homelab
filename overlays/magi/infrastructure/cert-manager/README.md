# Cert-Manager for Magi Cluster

Deploys cert-manager v1.20.2 via Helm. Configured for Let's Encrypt + Cloudflare DNS validation.

## Configure

1. `secret.yaml`: Add Cloudflare API token
2. `clusterissuer.yaml`: Add Let's Encrypt and Cloudflare emails
3. `certificate.yaml`: Update domains

```bash
kubectl apply -k overlays/magi/infrastructure/cert-manager/
```
