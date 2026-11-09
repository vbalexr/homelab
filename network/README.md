# Network

High-level description of how the homelab network and the MAGI cluster's routing are set up. Addresses, hardware inventory and
router configuration are kept out of the repository; NetBox is the source of truth for them.

## Segments

Separate VLANs for trusted clients, IoT, CCTV, network management and the lab (servers and cluster nodes), plus two routed-only ranges
that exist on no L2 segment: the Kubernetes LoadBalancer pool and the point-to-point links between the cluster nodes. Everything is
dual-stack (IPv4 and IPv6 ULA). A pair of routers in a CARP/MLAG arrangement does inter-VLAN routing and firewalling; a second,
separate network serves a remote site.

## MAGI cluster routing

The three MAGI Talos nodes use BGP for two independent jobs on two independent speakers, so the cluster keeps working even if the
routers are unreachable.

| Plane | Speaker | Carries |
|---|---|---|
| Cluster mesh | Talos native BGP (`fabric` instance), eBGP between the nodes over direct point-to-point links | Pod CIDRs and node loopbacks |
| External ingress | Cilium BGP Control Plane, peering the two routers over the lab VLAN | Service LoadBalancer IPs only |

- **Mesh:** each node has a direct link to each other node (no switch in the path), and a distinct ASN per node so a single dead link
  does not partition a pair. Sessions run over IPv6 and carry IPv4 prefixes too. Pod routes reach the mesh through a local VRF where
  Cilium peers with a second Talos instance (`workload`) that the `fabric` instance imports from.
- **Ingress:** the routers accept only exact host routes (/32 and /128) inside the LoadBalancer pool and reject everything else, and
  they peer with the nodes' real uplink addresses, not the CARP virtual address. Cilium never installs routes learned from the
  routers.
- **Uplink:** each node has an LACP bond on the lab VLAN for management, egress, the control-plane VIP and the router sessions. BGP
  never carries a default route.
- **LoadBalancer Services** use `externalTrafficPolicy: Local` where the real client address matters, so each address is advertised
  only from the node running the pod.

The router-side configuration needed to rebuild the peering is in [routers.md](routers.md). The deployed configuration is in `cluster/magi/talos/config/` (Talos) and `overlays/magi/infrastructure/cilium/` (Cilium), applied
by Flux.
