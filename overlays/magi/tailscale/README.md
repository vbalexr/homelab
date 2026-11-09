# Tailscale subnet router

Two replicas (spread over different nodes) join the tailnet and advertise the LoadBalancer pool, so tailnet devices reach everything
published there. Userspace networking: no TUN device, no extra capabilities.

## Setup

1. Generate an auth key that is reusable, not ephemeral and pre-approved, with a tag from the ACL `tagOwners`. It is only used on the
   first start of each replica; the node key is then kept in the Secrets `tailscale-0` and `tailscale-1`. If those Secrets are emptied,
   a valid auth key is needed again.
2. Store it: `sops overlays/magi/tailscale/subnet-router/secret.yaml` and replace `authkey`.
3. Approve the advertised subnets on each node (or add an `autoApprovers` rule for the tag).
4. Add the Technitium resolvers as a tailnet nameserver, with split DNS if they should not serve everything.
5. Clients that should use the routes: Linux needs `--accept-routes`; other platforms accept them by default.
