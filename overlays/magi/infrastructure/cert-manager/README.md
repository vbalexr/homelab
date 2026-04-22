# Cert-Manager for Magi Cluster

Deploys cert-manager via Helm (v1.20.2) from OCI registry, configured for Let's Encrypt + Cloudflare DNS validation.

## Configuration

### 1. Cloudflare API Token
Edit `secret.yaml` and replace `REPLACE_WITH_CLOUDFLARE_API_TOKEN` with your Cloudflare API token.

### 2. Email Addresses
Edit `clusterissuer.yaml`:
- Replace `REPLACE_WITH_EMAIL@example.com` — Let's Encrypt account email
- Replace `REPLACE_WITH_CLOUDFLARE_EMAIL@example.com` — Cloudflare account email

### 3. Domains
Edit `certificate.yaml`:
- Update `dnsNames` with your domains (currently: photos.vbalex.com, vault.vbalex.com)

## Deployment

Once configured, Flux CD will deploy automatically. Or deploy manually:

```bash
# Install HelmRepository source first
kubectl apply -f helmrepository.yaml

# Then apply the overlay
kubectl apply -k overlays/magi/infrastructure/cert-manager/
```

## Verification

```bash
# Check if cert-manager is running
kubectl -n cert-manager get pods

# Check ClusterIssuer status
kubectl get clusterissuer letsencrypt-prod

# Check certificate status
kubectl -n home get certificate home-tls
```

## How it works

1. **HelmRelease** deploys cert-manager via Helm with CRDs enabled
2. **ClusterIssuer** configures Let's Encrypt as the certificate authority
3. **Certificate** resource requests a cert for photos.vbalex.com and vault.vbalex.com
4. **Cloudflare DNS01** solver validates domain ownership
5. Secret `home-tls` is created in the home namespace with the TLS cert
6. **Gateway** references this secret for HTTPS termination
