# Tailscale subnet router

Two replicas (spread over different nodes) join the tailnet as ephemeral nodes `tailscale-<pod>` and advertise the
LoadBalancer pool `10.1.1.0/24` and `fd7a:2201:1ac::/64`. Tailnet devices then reach everything published there (DNS
resolvers `10.1.1.30`/`.31`, the gateways, databases). Userspace networking: no TUN device, no extra capabilities.

## Setup

1. Tailscale admin console, Settings > Keys: generate an auth key that is **reusable**, **ephemeral** and
   **pre-approved** (add a tag such as `tag:magi` if the ACLs use tags).
2. Store it: `sops overlays/magi/tailscale/subnet-router/secret.yaml` and replace `authkey`.
3. Machines tab: approve the two advertised subnets on each node (or add an `autoApprovers` rule for the tag in the ACL).
   Disable key expiry is not needed for ephemeral nodes.
4. DNS tab: add a nameserver `10.1.1.30` (and `10.1.1.31`) so tailnet devices resolve through Technitium; restrict it
   to the internal domains with split DNS if the resolvers should not serve everything.
5. Clients that should use the advertised routes: Linux needs `--accept-routes`; other platforms accept them by default.
