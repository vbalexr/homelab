# Technitium DNS

Two instances (`technitium-0`, `technitium-1`) of `docker.io/technitium/dns-server` as one StatefulSet in the `dns` namespace, each with
its own `longhorn-retain` volume on `/etc/dns`, on different nodes (required anti-affinity). The instances form a Technitium cluster
(primary and secondary); zones are created on the primary and replicate to the secondary.

Each instance has its own LoadBalancer Service serving DNS (53), DNS-over-TLS (853) and DNS-over-HTTPS (443), with a Let's Encrypt
certificate from cert-manager. The admin password is in the `technitium-credentials` secret. The web consoles are published through the
admin gateway, not on the DNS addresses.

The `cluster-sync` sidecar keeps the cluster registered with the current pod IPs.

## Update

The `technitium` image policy rewrites the tag in `kustomization.yaml`. The StatefulSet rolls one pod at a time, so one resolver always
answers.
