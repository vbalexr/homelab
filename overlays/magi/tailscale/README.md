# Tailscale subnet router

Two replicas (spread over different nodes) join the tailnet as the nodes `tailscale-0` and `tailscale-1` and advertise the
LoadBalancer pool `10.1.1.0/24` and `fd7a:2201:1ac::/64`. Tailnet devices then reach everything published there (DNS
resolvers `10.1.1.30`/`.31`, the gateways, databases). Userspace networking: no TUN device, no extra capabilities.

## Setup

1. Tailscale admin console, Settings > Keys: generate an auth key that is **reusable**, **not ephemeral** and
   **pre-approved**, with a tag such as `tag:magi` (defined in the ACL `tagOwners`). A tagged node's own key never expires.
   The key is only used on the first start of each replica; the node key is then kept in the Secrets `tailscale-0` and
   `tailscale-1`, so a later auth key expiry changes nothing. If those Secrets are emptied, a valid auth key is needed again.
2. Store it: `sops overlays/magi/tailscale/subnet-router/secret.yaml` and replace `authkey`.
3. Machines tab: approve the two advertised subnets on each node (or add an `autoApprovers` rule for the tag in the ACL). The nodes keep their names, so this is needed once.
4. DNS tab: add a nameserver `10.1.1.30` (and `10.1.1.31`) so tailnet devices resolve through Technitium; restrict it
   to the internal domains with split DNS if the resolvers should not serve everything.
5. Clients that should use the advertised routes: Linux needs `--accept-routes`; other platforms accept them by default.
