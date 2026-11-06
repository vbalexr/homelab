# Technitium DNS

Two instances (`technitium-0`, `technitium-1`) of `docker.io/technitium/dns-server` as one StatefulSet in `dns`, each with a
2Gi `longhorn-retain` volume on `/etc/dns`, on different nodes (required anti-affinity). Admin password in `technitium-credentials`
(`DNS_SERVER_ADMIN_PASSWORD`).

## Addresses and ports

| Instance | Name | LoadBalancer IPs | Ports |
|---|---|---|---|
| `technitium-0` | `dns0.vbalex.com` | `10.1.1.30`, `fd7a:2201:1ac::30` | 53 udp+tcp, 853 tcp (DoT), 853 udp (DoQ), 443 tcp (DoH) |
| `technitium-1` | `dns1.vbalex.com` | `10.1.1.31`, `fd7a:2201:1ac::31` | same |

`externalTrafficPolicy: Local`, so each address is advertised over BGP only from the node running that pod.

443 is served by the `proxy` container (nginx-unprivileged, `proxy.conf`, certificate from `technitium-tls`, reloaded every 6 h to
pick up renewals): `https://dns0.vbalex.com/admin/` (and `dns1`) is the admin console, and everything else goes to Technitium's
cleartext DNS-over-HTTP listener on `127.0.0.1:80` (DoH is wire format at `/dns-query`, GET and POST; the `application/dns-json`
form redirects to the help page). Technitium's own DoH-over-TLS is disabled (`enableDnsOverHttps=false`,
`enableDnsOverHttp=true`, `dnsReverseProxyNetworkACL=127.0.0.1,::1` on both nodes) so the proxy can own 443. The console uses
relative URLs, which is why it works under `/admin/`. The cluster API (53443) is not exposed. Nothing listens for DoH over HTTP/3.

`dns0`/`dns1` resolve to those addresses in Technitium (one primary zone per name, in the cluster catalog) and as host overrides
(A and AAAA) in the OPNsense Unbound overrides on Victim, synced to Demiurg.

## Update

The `technitium` image policy (`>=15.6.2 <16.0.0`) rewrites the tag in `kustomization.yaml`. The StatefulSet rolls one pod
at a time (`technitium-1`, then `technitium-0`), waiting for readiness plus `minReadySeconds: 30`, so one resolver always answers.

## Certificate

`certificate.yaml` issues `dns0.vbalex.com` and `dns1.vbalex.com` from `letsencrypt-prod` (DNS-01 through Cloudflare) into the
`technitium-tls` secret, including a pkcs12 keystore protected by `technitium-tls-password`. It is mounted at `/etc/dns-tls` in
both pods (optional volume, so a pod starts before the first issuance). The cluster API on 53443 keeps its own self-signed
certificate; the nodes authenticate each other with DANE-EE (TLSA records in the cluster zone), so that one needs no trusted CA.

## Cluster

Cluster domain `dns.vbalex.com`; node names `technitium-N.dns.vbalex.com`; `technitium-0` is the primary, `technitium-1` the
secondary. Zones are created on the primary with `catalog=cluster-catalog.dns.vbalex.com` and reach the secondary through the
catalog. Settings, users, blocklists and allow/block lists sync too; the optional DNS protocol settings below do not.

The cluster identifies nodes by IP address (zone transfers, notifications, DANE), and pod IPs change on every restart. The
`cluster-sync` sidecar (`cluster-sync.sh`, same image) checks every 15 s that the IP each node is registered with equals its pod IP
and re-registers with `cluster/updateIpAddress` when not; a secondary that cannot reach the primary points itself at the primary's
current pod IP with `cluster/secondary/updatePrimary`. Logs: `kubectl -n dns logs technitium-N -c cluster-sync`.

Setup from scratch (API on a port-forward, `$T0`/`$T1` login tokens, `$PW` the admin password):

```sh
# 1. server domain per node (the DNS_SERVER_DOMAIN env var only applies to the very first start)
curl localhost:5380/api/settings/set -d token=$T -d dnsServerDomain=technitium-N.dns.vbalex.com
# 2. primary
curl localhost:5380/api/admin/cluster/init -d token=$T0 -d clusterDomain=dns.vbalex.com -d primaryNodeIpAddresses=<pod IP of technitium-0>
# 3. secondary (ignoreCertificateErrors only for this call)
curl localhost:5381/api/admin/cluster/initJoin -d token=$T1 -d secondaryNodeIpAddresses=<pod IP of technitium-1> \
  -d primaryNodeUrl=https://technitium-0.dns.vbalex.com:53443/ -d primaryNodeIpAddress=<pod IP of technitium-0> \
  -d ignoreCertificateErrors=true -d primaryNodeUsername=admin --data-urlencode primaryNodePassword=$PW
```

If a join leaves the secondary refused or "Unreachable" (registered IP differs from the pod IP), fix the address from the
primary: `cluster/primary/updateSecondary` with the secondary's node id, URL, pod IP and its web-service certificate
(base64url DER from `openssl s_client -connect <pod>:53443`), then `cluster/secondary/resync` on the secondary.

## Per-node settings (not synced)

Set on both nodes after the certificate exists:

```sh
curl localhost:5380/api/settings/set -d token=$T -d dnsTlsCertificatePath=/etc/dns-tls/keystore.p12 \
  --data-urlencode dnsTlsCertificatePassword=$(kubectl -n dns get secret technitium-tls-password -o jsonpath='{.data.password}' | base64 -d) \
  -d enableDnsOverTls=true -d enableDnsOverQuic=true \
  -d enableDnsOverHttps=false -d enableDnsOverHttp=true -d dnsOverHttpPort=80 -d dnsReverseProxyNetworkACL=127.0.0.1,::1
```

## Overrides

Mirrors the OPNsense Unbound host overrides so VLANs using these resolvers see the same split-horizon answers: one primary zone
per name with an A record (assistant, mqtt, cameras, photos, minecraft, vault, ai, esphome, netbox `.vbalex.com`;
`auth.suspectsquadron.co.uk`, `auth.mrakaki.com`, `buyback.mrakaki.com`) plus `dns0`/`dns1` with A and AAAA, and a conditional
forwarder zone `juahu.com` to the router (`10.1.0.1`) for the DHCP hostnames. A zone per name overrides only that name; the
rest of the parent domain still resolves publicly.
