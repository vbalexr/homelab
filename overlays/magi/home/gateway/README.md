# Cilium Gateway API

Modern Kubernetes Gateway API implementation for routing to home namespace apps.

## How it works

- **Gateway** (`home-gateway`): Listens on ports 80 (HTTP) and 443 (HTTPS), creates a LoadBalancer service
- **HTTPRoutes**: Route by hostname:
  - `photos.vbalex.com` → immich:2283
  - `vault.vbalex.com` → vaultwarden:80

## Dual-stack

The Gateway creates a dual-stack LoadBalancer automatically. Cilium will:
1. Assign IPv4 address from `public-pool`
2. Assign IPv6 address from `public-pool`
3. Advertise both via BGP

## DNS

Point both domains to the same LoadBalancer IP:
```bash
photos.vbalex.com    A/AAAA    <LoadBalancer-IP>
vault.vbalex.com     A/AAAA    <LoadBalancer-IP>
```

## HTTPS

HTTPS is automatically managed by cert-manager:

1. **Certificate resource** (`certificate.yaml`): Requests a TLS certificate from Let's Encrypt for both domains
2. **ClusterIssuer** (`letsencrypt-prod`): Validates domain ownership via Cloudflare DNS01
3. **Secret creation**: cert-manager automatically creates the `home-tls` secret in the home namespace
4. **Gateway termination**: The gateway uses this secret for HTTPS termination

### Verification

Check certificate status:
```bash
kubectl -n home get certificate home-tls
kubectl -n home describe certificate home-tls
```

Check the secret was created:
```bash
kubectl -n home get secret home-tls
```

Certificate will auto-renew 30 days before expiration.
