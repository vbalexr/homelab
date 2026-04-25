# Cilium Gateway API

Gateway API implementation for routing to home namespace apps.

- Gateway: Listens on ports 80 (HTTP), 443 (HTTPS)
- Routes: photos.vbalex.com → immich:2283, vault.vbalex.com → vaultwarden:80
- HTTPS: Auto-managed by cert-manager

Point DNS A/AAAA records to LoadBalancer IP.
