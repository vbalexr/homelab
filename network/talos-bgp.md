# MAGI — BGP design

Applies to the **MAGI** Talos nodes (Balthasar / Casper / Melchior) on the Home site. Evidence and
sources behind every claim here are in [`cluster/magi/README.md`](../cluster/magi/README.md).
The Talos mesh (`fabric` instance) is deployed from `cluster/magi/talos/config/`. Cilium, the `workload` instance and the router peering are designed here but not deployed yet.

---

## 1. Design in one view

BGP does two independent jobs, on two independent speakers:

| Plane | Speaker | Peers | Carries | Needs routers/switches? |
|---|---|---|---|---|
| **Cluster mesh** | Talos native BGP, instance `fabric` | The other two nodes, over direct DAC `/31`s | Pod CIDRs, node `/32`s | **No** — cluster keeps working fully isolated |
| **External ingress** | Cilium BGP Control Plane | Victim and Demiurg (real addresses, not the CARP VIP) | LoadBalancer IPs only | Yes — only external reachability depends on it |

```
              Victim (65000)          Demiurg (65000)
                    ▲  eBGP, VLAN 10, LB /32s only  ▲
                    └──────────┬──────────────┬─────┘
                               │ (uplink bond: Aura + Mare MLAG, VLAN 10)
        ┌──────────────────────┼──────────────────────┐
   Balthasar 65011        Casper 65012           Melchior 65013
        │  ╲                   │                   ╱  │
        │   ╲_______ DAC /31 mesh, eBGP, 9 s hold ____╱  │
        └────────────────────────────────────────────┘
   per node:  Cilium (65021) ──veth── Talos instance "workload" (65031, VRF, no route install)
                                              │ importRoutes: podSubnets only
                                              ▼
                                        Talos instance "fabric"
```

Rules that follow from this:

* The mesh instance has **no router neighbor**. Talos has no export policy, so anything peered with it
  would receive pod CIDRs and node `/32`s, and anything it sent would be installed in the node's main
  table.
* Routers are peered by **Cilium**, whose advertisement is selected per peer: the router peer sees
  Service LoadBalancer IPs, the Talos `workload` peer sees the PodCIDR. Cilium never installs received
  routes, so nothing from the routers reaches the node tables.
* The default route, management and egress use the plain L2 uplink on VLAN 10. BGP never carries a
  default, so a node with a dead uplink cannot use a peer as its gateway by accident.

---

## 2. Native BGP in Talos 1.14

Talos **1.14.0** added an embedded GoBGP speaker configured with repeatable `BGPInstanceConfig`
documents. Current stable is **1.14.4**.

| Object | Role |
|---|---|
| `BGPInstanceConfig` | One named BGP RIB + speaker, bound to the default routing domain **or** one VRF. |
| `VRFConfig` / `VethConfig` / `LinkConfig` | Routing domains and links the instance binds to. |
| `BGPPeerStatus` | Read-only session state, prefix counts, BFD state; IDs like `fabric/10.1.3.1`. |

Instance fields: `name`, `vrf`, `localASN`, `routerID` (IPv4), `routeSource`, `advertise` (links whose
addresses are originated), `multipath`/`maxPaths`, `installRoutes`, `importRoutes[{bgpInstance, prefixes}]`,
`neighbors[]`. Neighbor fields: `address` (numbered) or `link` (unnumbered), `peerASN`, `localASN`,
`passive`, `holdTime`, `bfd`.

Per instance Talos originates addresses of `advertise`d links, establishes sessions, installs learned
best paths into the Linux FIB (unless `installRoutes: false`; originated and imported prefixes are never
installed), and publishes `BGPPeerStatus`.

Limits that shape the design:

* One instance per default routing domain, one per VRF.
* BFD only in the default routing domain (not VRF-bound instances).
* Unnumbered peers need a router that sends IPv6 RAs; Talos nodes do not ⇒ **node↔node links are numbered**.
* No route reflector, no export policy, no local-pref/MED/community/prepend/prefix-list.
* `importRoutes`: prefixes must be masked, must not overlap, each source instance once, no self-import,
  IPv4/IPv6 unicast only.
* Equal best paths become one ECMP route when `multipath: true`.

---

## 3. Addressing and ASNs

| Item | Value |
|---|---|
| Mesh links (point-to-point, not advertised) | `10.1.3.0/31` + `fd7a:2201:1ad::/127` Balthasar ↔ Casper · `10.1.3.2/31` + `::2/127` Balthasar ↔ Melchior · `10.1.3.4/31` + `::4/127` Casper ↔ Melchior |
| Node identity, on `lo`, advertised | Balthasar `10.1.3.11/32` · Casper `10.1.3.12/32` · Melchior `10.1.3.13/32`, and `fd7a:2201:1ad::11` / `::12` / `::13` as `/128` (IPv4 also the router IDs) |
| Uplink (VLAN 10, not advertised) | Balthasar `10.1.0.11` / `fd7a:2201:1ab::11` · Casper `.12` / `::12` · Melchior `.13` / `::13`, LACP bond `bond-up` (`enp5s0` + `enp4s0`) |
| Control-plane VIP | `10.1.0.10`, `magi.vbalex.com` — Talos `Layer2VIPConfig` on VLAN 10 (IPv4 only: the L2 VIP announces with gratuitous ARP) |
| LB pool (BGP-only, no L2) | `10.1.1.0/24`, `fd7a:2201:1ac::/64` |
| Pod / service CIDRs | `10.244.0.0/16` + `fd7a:2201:1ae::/56` · `10.96.0.0/12` + `fd7a:2201:1af::/112` (dual-stack) |
| Routers | Victim `10.1.0.2` / `fd7a:2201:1ab::2` and Demiurg `10.1.0.3` / `fd7a:2201:1ab::3` (CARP VIP `.1` is **not** a peer) |

| Entity | ASN |
|---|---|
| Victim, Demiurg | `65000` |
| Talos `fabric`: Balthasar / Casper / Melchior | `65011` / `65012` / `65013` (distinct per node) |
| Cilium (all nodes) | `65021` |
| Talos `workload` (all nodes, local sessions only) | `65031` |

Distinct `fabric` ASNs are mandatory. With one shared ASN (iBGP) a node would not re-advertise a route
learned from one peer to the other, so a single dead cable would partition that pair; with eBGP the
third node transits.

---

## 4. Physical layout

* **East-west:** each node has two DAC/AOC ports, one to each other node. MTU 1500, same as every other link. No switch in the path.
* **North-south:** each node has LACP bond `bond-up` (`enp5s0` + `enp4s0`) on VLAN 10 for management, egress, the control-plane
  VIP and the router BGP sessions.

---

## 5. Talos machine config

Deployed config: `cluster/magi/talos/config/common.yaml` and `<node>.yml` (hostname, install disk, `lo`, `bond-up`
uplink, mesh links, `fabric` and `workload` instances, the veth/VRF pair). Cilium's BGP resources are in
`overlays/magi/infrastructure/cilium/` (`bgp-pods.yaml`, `bgp-cluster.yaml`, `bgp-routers.yaml`, `load-balancer-ip-pool.yaml`), applied by Flux.

* **Mesh sessions are IPv6-only**, one per peer over the `/127`. IPv4 prefixes (node `/32`s, pod IPv4 CIDRs)
  ride them with IPv6 next hops, which Linux accepts. With an IPv4 session per peer as well, an IPv6 prefix
  learned over it gets an IPv4 next hop, and the kernel rejects the whole ECMP route (`invalid argument`), so
  no IPv6 route is installed.
* **No BFD on the mesh.** BFD over IPv6 stays `down` in Talos 1.14.4 (IPv4 BFD works), so failover relies on the
  BGP hold time (9 s).
* **Pod routing:** every node's Cilium advertises its PodCIDRs to the local Talos `workload` instance over the
  veth `veth-workload` (host) ↔ `veth-router` (in `vrf-workload`); `fabric` imports `10.244.0.0/16` and
  `fd7a:2201:1ae::/56` from it and advertises them to the mesh, where the peers install them in the main table.

Cluster-level settings that go with it:

* etcd `advertisedSubnets` and kubelet `nodeIP.validSubnets` pinned to `10.1.3.8/29` and `fd7a:2201:1ad::10/124` (the `lo` identities),
  so cluster traffic uses the mesh and not the uplink. Without this the kubelet picks its node IP from the
  podCIDRs and may choose the wrong NIC.
* KubePrism enabled (port 7445); Cilium uses `k8sServiceHost: localhost`, `k8sServicePort: 7445`.
* Talos ingress firewall: allow TCP/179 on the mesh links and on the veth.
* Cilium prerequisites for Talos: `ipam.mode: kubernetes`, no cgroup/bpffs mounts by Cilium, no
  `SYS_MODULE`, Flannel disabled with `KubeFlannelCNIConfig` `$patch: delete`, and
  `forwardKubeDNSToHost` / `bpf.hostLegacyRouting` set per the Sidero Cilium guide.

---

## 6. Cilium

Helm values (key ones):

```yaml
routingMode: native
ipv4NativeRoutingCIDR: <cluster.network.podSubnets>
autoDirectNodeRoutes: false        # nodes are on different L2 segments (point-to-point links)
bgpControlPlane:
  enabled: true
ipam:
  mode: kubernetes
k8sServiceHost: localhost
k8sServicePort: 7445
```

BGP resources, one `CiliumBGPClusterConfig` instance (`localASN: 65021`) per node with three kinds of peer:

| Peer | Address | ASN | Advertisement selected |
|---|---|---|---|
| Talos `workload` | `fda1:b2c3:d4e5:1::1` (`sourceInterface: veth-workload`) | 65031 | `PodCIDR` |
| Victim | `10.1.0.2` and `fd7a:2201:1ab::2` | 65000 | Service `LoadBalancerIP` |
| Demiurg | `10.1.0.3` and `fd7a:2201:1ab::3` | 65000 | Service `LoadBalancerIP` |

* `CiliumLoadBalancerIPPool` blocks: `10.1.1.0/24`, `fd7a:2201:1ac::/64`.
* Services that should be reachable externally use `externalTrafficPolicy: Local`; only nodes with a local
  backend advertise the VIP.
* One peer entry per router per address family.
* Router sessions run over a shared L2 segment, so a node whose uplink fails does not drop the router's
  interface. Detection is by BGP timers: set short `holdTimeSeconds` / `keepAliveTimeSeconds` on the router
  `CiliumBGPPeerConfig` (and a matching timer or BFD on the router side) instead of the 90 s default.

---

## 7. Router side (Victim, Demiurg)

Both routers peer with every node's **uplink** address, never over a CARP VIP.

* **Accept** only exact host routes inside the LB pools: `/32` within `10.1.1.0/24`, `/128` within
  `fd7a:2201:1ac::/64`. Reject everything else, including covering prefixes and pod CIDRs.
* **Advertise nothing** to the nodes.
* Configure the BGP sessions on **both** routers and keep them established regardless of CARP master/backup
  state; do not assume the FRR configuration is synced between them. Each node peers with both, so losing
  one router leaves every VIP reachable.
* Enable multipath/ECMP so a VIP advertised by several nodes load-balances; add a Null0/blackhole route
  for the pool prefixes so they do not loop when no node advertises them.
* Generic FRR shape (OPNsense `os-frr`):

```
ip prefix-list MAGI-LB4 seq 10 permit 10.1.1.0/24 ge 32 le 32
ipv6 prefix-list MAGI-LB6 seq 10 permit fd7a:2201:1ac::/64 ge 128 le 128
route-map MAGI-IN permit 10
 match ip address prefix-list MAGI-LB4
route-map MAGI-IN6 permit 10
 match ipv6 address prefix-list MAGI-LB6
route-map MAGI-OUT deny 10
router bgp 65000
 neighbor MAGI peer-group
 neighbor MAGI remote-as 65021
 neighbor 10.1.0.11 peer-group MAGI
 neighbor 10.1.0.12 peer-group MAGI
 neighbor 10.1.0.13 peer-group MAGI
 address-family ipv4 unicast
  neighbor MAGI route-map MAGI-IN in
  neighbor MAGI route-map MAGI-OUT out
  maximum-paths 3
```

The IPv6 family is configured the same way over IPv6 neighbors (`fd7a:2201:1ab::11`–`::13`) with
`MAGI-IN6`.

---

## 8. Behavior and failure matrix

* **Mesh paths:** a peer's prefixes arrive directly as a 1-hop AS-path and via the third node as 2-hop.
  The direct path wins automatically; the transit path is the fallback. No router is in the path, so
  there are no ties.
* **Pod routes:** each node installs remote PodCIDRs and node `/32`s learned from peers into its main table.
  Its own prefixes are never installed (local wins, and imported/originated prefixes are skipped).

| Failure | Effect |
|---|---|
| One DAC cable or port | The hold timer (9 s) drops that session; the pair reaches each other via the third node. |
| Both DACs of one node | Its mesh routes are withdrawn after the hold timer; the metric-2000 static routes on `bond-up` (peers' `lo` and PodCIDRs via their uplink addresses) carry pod, etcd and kubelet traffic over the 2.5G uplink. |
| One node down | Peers withdraw its prefixes; pods reschedule; etcd keeps quorum (2 of 3). |
| One node's uplink down | Its router sessions drop and the routers withdraw its VIPs. Its pods stay reachable through the mesh. |
| Aura or Mare down | The MLAG bond keeps the other member. No effect on the mesh. |
| Aura↔Mare peer link down | MLAG degrades; external ingress/egress may be affected. Mesh unaffected. |
| Victim or Demiurg down | The other router still holds a session to every node, so ingress continues. Cluster-internal traffic is unaffected. |
| Both routers down | External LB reachability is lost; the cluster is unaffected. |
| All routers and switches down | Cluster fully functional internally; no external access. The L2 control-plane VIP is unavailable, in-cluster clients use KubePrism. |
| Talos BGP bug or crash on a node | That node loses mesh routes. Management stays reachable on the VLAN 10 uplink. |

---

## 9. Verified on hardware

* **Pod routing:** pods on different nodes reach each other over IPv4 and IPv6 (0% loss), DNS and egress work.
  Each node holds 8 BGP routes (both peers' `lo` and pod CIDRs, both families).
* **One DAC pulled (Balthasar–Casper), under iperf3 load:** the BGP session drops 8 s after the link goes down
  (9 s hold time, no BFD on the IPv6 sessions) and the pair reaches each other through Melchior, so a Talos
  node does re-advertise learned routes. Streams on the cut path stall 26 to 32 s (TCP retransmit backoff)
  and then resume at full rate; streams on the other path are unaffected. Total throughput into the server
  falls from 13.1 to 7.9 Gbit/s while it sits behind one DAC. The direct session returns about 1 minute after
  the cable is back; traffic stays on the transit path meanwhile.
* **Both DACs pulled on one node:** both sessions drop, the other two keep peering, and etcd re-elects a
  leader in about 2 s when the isolated node was the leader.
* **Single cable cut under load, after the IPv6 default route metric fix:** pings black-holed for 8 s (down from
  49 s), Casper streams stalled 7 to 12 s, fail-back lost one ping. The earlier 49 s came from a stalled route
  controller, not from BGP.
* **Both DACs of one node out, with the uplink static routes:** pings black-holed for about 9 s (hold timer),
  then pod traffic crossed the uplink at 2.3 to 2.4 Gbit/s on IPv4 and IPv6, nodes stayed Ready and etcd kept
  all three members. After replug the BGP sessions returned in 7 to 9 s with no lost pings and the mesh routes
  took over again. Before the static routes this case stopped pod traffic completely and Balthasar's etcd lost
  its leader.
* **All three nodes rebooted at once:** all back, etcd and the mesh re-form, nodes Ready after about 4 min.
* **`advertise`:** a `lo` `/32` is advertised as a `/32`.

## 10. Still to validate

1. **Port 179.** The Talos GoBGP and the Cilium agent both run BGP in the host namespace; confirm they do not
   collide.
3. **Cold boot, isolated.** Unplug all uplinks, power all three nodes at once; etcd must form over the
   `lo` addresses.
4. **Pinned PodCIDRs.** The uplink static routes hard-code each node's PodCIDR; allocation order is not fixed.
5. **Router sessions.** Cilium ↔ Victim/Demiurg: only the accepted prefixes, nothing sent back, ECMP across
   nodes, VIP withdrawn when a node's backend or uplink disappears.

Open items:

* Victim and Demiurg (OPNsense 26.7.7) run `os-frr` with the §7 policy, configured as in §10a.
* Cilium version to pair with Talos 1.14 and the `transport.sourceInterface` requirement.

---

## 10a. Router configuration as deployed

Victim and Demiurg each run FRR (`os-frr`) as AS 65000, peering every node's static uplink address over IPv4 and IPv6
(12 sessions in total, hold 9 s / keepalive 3 s). The two routers are configured independently (no HA config sync) with the
same settings; only the router ID differs. Configure each in the OPNsense GUI:

**Routing → General**
* Enable routing; turn **off** "Enable CARP Failover" so the sessions stay up on both routers.
* Turn **off** the automatic firewall rules: the "Pass BGP" rule they create opens TCP/179 on every interface, WAN
  included. The existing LAB "pass any" rules already allow the nodes to reach the routers.

**Routing → BGP → General**
* Enable BGP, AS number `65000`, router ID `10.1.0.2` (Victim) / `10.1.0.3` (Demiurg), maximum paths `3`, log neighbor changes.

**Prefix lists**

| Name | Version | Seq | Action | Network |
|---|---|---|---|---|
| `MAGI-LB4` | IPv4 | 10 | permit | `10.1.1.0/24 ge 32 le 32` |
| `MAGI-LB6` | IPv6 | 10 | permit | `fd7a:2201:1ac::/64 ge 128 le 128` |

**Route maps**

| Name | Action | ID | Match |
|---|---|---|---|
| `MAGI-IN4` | permit | 10 | prefix list `MAGI-LB4` |
| `MAGI-IN6` | permit | 10 | prefix list `MAGI-LB6` |
| `MAGI-OUT` | deny | 10 | none |

**Neighbors** (six, all remote AS `65021`, keepalive `3`, hold `9`, route map out `MAGI-OUT`)

| Address | Node | Route map in | Update source |
|---|---|---|---|
| `10.1.0.11` / `.12` / `.13` | Balthasar / Casper / Melchior | `MAGI-IN4` | LAB |
| `fd7a:2201:1ab::11` / `::12` / `::13` | Balthasar / Casper / Melchior | `MAGI-IN6` | none |

Then save and apply. The rendered `/usr/local/etc/frr/frr.conf` contains `router bgp 65000`, the six `neighbor` blocks,
`maximum-paths 3` in both address families, the two prefix lists and the three route maps.

* Check: `vtysh -c 'show bgp summary'` on each router shows six established sessions; `cilium-cli bgp peers` on the cluster.
* Each node pins the source address of its router sessions (`CiliumBGPNodeConfigOverride`): the uplink also holds a
  SLAAC address that the kernel prefers, and the routers only accept the static ones.
* Demiurg has no IPv4 default route, so `pkg install os-frr` cannot reach the repository from it. Create the packages
  on Victim (`pkg create -o <dir> os-frr frr10 frr10-pythontools c-ares libssh libunwind libyang3 net-snmp protobuf-c
  protobuf abseil jsoncpp rtrlib`), copy them over `ssh` (no sftp on Demiurg) and `pkg add -f` them in dependency
  order, then `service configd start`.
* Not configured yet: a Null0/blackhole route for the two pool prefixes.

---

## 11. Version facts

* Talos 1.14.4 (stable); Kubernetes 1.33–1.37 supported; embedded GoBGP v4.8.0.
* Installer images come from Image Factory (`factory.talos.dev/metal-installer/<schematic>`) only.
* Cilium 1.20.4 is the current release; the Sidero Cilium guide renders 1.18.0.

---

## Links

* Native BGP overview — <https://docs.siderolabs.com/talos/v1.14/networking/bgp/overview>
* `BGPInstanceConfig` reference — <https://docs.siderolabs.com/talos/v1.14/reference/configuration/network/bgpinstanceconfig>
* Cilium BGP with Talos — <https://docs.siderolabs.com/talos/v1.14/networking/bgp/cilium>
* Deploying Cilium on Talos — <https://docs.siderolabs.com/kubernetes-guides/cni/deploying-cilium>
* Virtual (shared) IP — <https://docs.siderolabs.com/talos/v1.14/networking/advanced/vip>
* Cilium BGP Control Plane — <https://docs.cilium.io/en/latest/network/bgp-control-plane/bgp-control-plane-configuration/>
* Cilium routing modes — <https://docs.cilium.io/en/stable/network/concepts/routing/>
* pfSense FRR BGP — <https://docs.netgate.com/pfsense/en/latest/packages/frr/bgp/configuration.html>
* OPNsense FRR — <https://docs.opnsense.org/manual/dynamic_routing.html>
