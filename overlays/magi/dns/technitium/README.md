# Technitium DNS

Two instances (`technitium-0`, `technitium-1`) of `docker.io/technitium/dns-server` as one StatefulSet in `dns`, each with a
2Gi `longhorn-retain` volume on `/etc/dns`, on different nodes (required anti-affinity). Admin password in `technitium-credentials`
(`DNS_SERVER_ADMIN_PASSWORD`); each instance's `DNS_SERVER_DOMAIN` is `<pod>.technitium-headless.dns.svc.cluster.local`.

Resolver addresses (LoadBalancer, 53/udp and 53/tcp, `externalTrafficPolicy: Local`): `10.1.1.30` / `fd7a:2201:1ac::30` and
`10.1.1.31` / `fd7a:2201:1ac::31`. The admin console (5380) is the `technitium` ClusterIP Service:
`kubectl -n dns port-forward svc/technitium 5380`.

Updates: the `technitium` image policy (`>=15.6.2 <16.0.0`) rewrites the tag in `kustomization.yaml`. The StatefulSet rolls one pod
at a time (`technitium-1`, then `technitium-0`), waiting for readiness plus `minReadySeconds: 30`, so one resolver always answers.
