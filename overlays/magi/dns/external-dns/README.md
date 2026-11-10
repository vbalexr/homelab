# external-dns

Creates the DNS records of the HTTPRoute hostnames in the `vbalex.com` and `mrakaki.com` zones of Technitium. Records point at the address of the
Gateway the route is attached to. Only `vbalex.com` and `mrakaki.com` are managed; routes for other domains are ignored.

## Set up (once)

1. In Technitium (primary node), for each of the `vbalex.com` and `mrakaki.com` zones: Options, Dynamic Updates, policy allow for TSIG key `external-dns`
   on the zone (all record types), and under Zone Transfer allow the same key (external-dns lists the zone with AXFR). Create the key under Settings, TSIG, algorithm `hmac-sha256`, and copy its secret.
2. Create the secret and encrypt it:

   ```sh
   kubectl create secret generic external-dns-tsig -n dns --from-literal=tsig-secret=<key> --dry-run=client -o yaml \
     > overlays/magi/dns/external-dns/secret.yaml
   sops --encrypt --in-place overlays/magi/dns/external-dns/secret.yaml
   ```
3. Uncomment `secret.yaml` in `kustomization.yaml` and push.

Updates are sent to `technitium-node-0`, which must be the primary of the Technitium cluster. If the primary moves to node 1,
change `--rfc2136-host` in `helmrelease.yaml`.
