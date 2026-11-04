# MAGI — Talos native BGP + Cilium: findings and where they came from

Researched **2026-10-23**, revised **2026-10-24** for the scoped design. This file records **what was established and which source established it**, so
the design decisions in [`network/talos-bgp.md`](../../network/talos-bgp.md) can be re-checked or
challenged. Design YAML lives there; this file is the evidence log.

Target topology: 3 MAGI nodes joined by a **direct DAC mesh** with Cilium in **native routing** mode and
Talos native BGP writing the kernel routes — this plane is **cluster-only** and works with every switch and
router gone. Each node also has an LACP bond into the Aura/Mare MLAG pair (VLAN 10) for management, egress and
the control-plane VIP. Load-balancer IPs are advertised to Victim/Demiurg by **Cilium's own BGP**, separate
from the Talos mesh.

---

## 1. Method

| Method | What it was used for |
|---|---|
| Read Sidero docs pages in full (`fetch`), not search snippets | Documented behaviour, config reference, limitations |
| Read Talos source at **tag `v1.14.4`** from `raw.githubusercontent.com` | Behaviour that docs do not state (prefix masking, route installation, ECMP, peer resolution, validation) |
| `GET api.github.com/repos/siderolabs/talos/commits/2193b5781` | The 77-file change list of the native-BGP commit, to find the right files |
| `GET api.github.com/repos/siderolabs/talos/git/trees/v1.14.4?recursive=1` | Locate BGP/VRF/veth/bond config types and integration tests |
| Read Talos **integration tests** (`internal/integration/api/bgp.go`, `bgp_workload.go`) | The closest thing to executable proof of installed routes, hop counts and anycast VIP |
| Read GoBGP source (`osrg/gobgp`, master) | Best-path comparison and multipath rule, which determine the failover ladder |
| Read Cilium docs + Cilium GitHub issues | What Cilium's BGP control plane does **not** do |
| Search-engine snippets only | MikroTik licensing/hardware specs, pfSense/OPNsense FRR — flagged as such below |

Pinned versions checked: **Talos `v1.14.4`** (latest stable patch listed on GitHub releases on 2026-10-23;
`v1.14.4` released 2026-10-16, `v1.13.12` 2026-10-21 on the previous line), Talos pins
**`github.com/osrg/gobgp/v4 v4.8.0`** (checked in Talos `go.mod`), Cilium **1.20.4** (GitHub releases) while
the Sidero Cilium guide renders charts at **1.18.0** — version pairing for MAGI is unresolved (see §5).

---

## 2. Claims and their evidence

Confidence: **docs** = stated by upstream documentation · **source** = read in code at a pinned tag ·
**test** = asserted in an upstream integration test · **issue** = upstream issue tracker record ·
**inferred** = concluded from implementation behaviour, not written down upstream.

| # | Claim | Evidence | Type |
|---|---|---|---|
| 1 | Native BGP landed in **Talos 1.14.0** (released 2026-09-03) as repeatable, named `BGPInstanceConfig` documents, backed by an embedded GoBGP server; it was built to remove the FRR system extension for the fabric-facing case | `github.com/siderolabs/talos/releases/tag/v1.14.0` ("Native BGP" section); docs `talos/v1.14/networking/bgp/overview`; commit `2193b5781` "feat: native BGP support via embedded GoBGP" (2026-06-21, +13331/−5221, 77 files); design discussion `siderolabs/talos#11126` | docs, source |
| 2 | The config kind was **renamed during the release cycle**: `BGPConfig`/`BGPPeerConfig` in `v1.14.0-beta.0`, `BGPInstanceConfig` by `1.14.0-rc.1/2` | `v1.14.0-beta.0` release notes name `BGPPeerConfig`; the commit's file list contains `.../resources/network/bgp_peer_config.go` and `website/content/v1.14/reference/configuration/network/bgppeerconfig.md`; `v1.14.4` tree contains only `bgp_instance_config.go` | source |
| 3 | **Only one instance may own the default routing domain, and one per VRF**; a VRF-bound instance installs into the VRF table only | docs "Limitations" (`.../bgp/overview`); `internal/bgp/runtime.go` `resolveLinks()` → `validateLinkDomain()` on advertised links and neighbors | docs, source |
| 4 | `advertise` originates each address on a listed link **masked** — a link address `198.18.128.1/17` is advertised as `198.18.128.0/17`, and the `/32` host route is *not* advertised | `internal/bgp/runtime.go` `connectedPrefixes()` → `address.Masked()`; and asserted end-to-end in `internal/integration/api/bgp.go` `assertConnectedPrefixOrigination()`: witness node must learn `advertisedNetworks = {ipv4.Masked(), ipv6.Masked()}` and `assertLearnedRouteAbsent(hostRoutes)` | source, test |
| 5 | Addresses that are loopback or link-local are skipped when collecting what to advertise | `internal/bgp/runtime.go` `advertisedAddresses()`: `if addr.IsLoopback() \|\| addr.IsLinkLocalUnicast() \|\| ... { continue }` | source |
| 6 | `routerID` must be IPv4, else derived from the first advertised IPv4 address | docs; `internal/bgp/runtime.go` `routerID()`; `Validate()` in `pkg/machinery/config/types/network/bgp.go` | docs, source |
| 7 | Routes **learned from neighbors** are installed into the Linux FIB (`RouteSpec`, `Protocol: bgp`, `ConfigLayer: ConfigOperator`) unless `installRoutes: false`; prefixes the instance **originated or imported are explicitly excluded** from installation | docs "What Talos does per instance"; `internal/bgp/instance.go` `learnedRoutes()` builds `localSet` from `instance.advertised` **and** `instance.imported` and skips any destination in it | docs, source |
| 8 | Several equal best paths become a **single ECMP `RouteSpec`** (`NextHops[]`), canonicalised for stable reconciles | `internal/bgp/instance.go` `learnedRoutes()` appends every `path.Best` next-hop, then `network.NormalizeNextHops()` with the comment "several best paths may share a next-hop … canonicalize the ECMP set"; `internal/bgp/bgp.go` `RouteSpec()` sets `NextHops` when `len(nexthops) > 1` | source |
| 9 | Upstream proves the ECMP-installed learned route: a witness/worker node installs the destination "via the expected number of IPv6-link-local next-hops (RFC 8950), one per fabric link" | `internal/integration/api/bgp.go` `assertLearnedRoute(nodeCtx, dest, hops)` checks `spec.Protocol == ProtocolBGP` and `len(spec.NextHops) == hops`; used for both connected prefixes and the anycast VIP | test |
| 10 | GoBGP best-path comparison order: **local-injected > learned**, eBGP > iBGP, higher local-pref, **shorter AS-path**, better origin, higher MED — and **there is no router-ID tie-break** | `osrg/gobgp` `internal/pkg/table/path.go` `(*Path).Compare()` (checked against master; Talos pins v4.8.0) | source |
| 11 | Multipath candidates are the paths whose `Compare()` against the best returns **exactly 0** (i.e. equal on every criterion above) | `osrg/gobgp` `internal/pkg/table/destination.go` `Select()` with `MultiPath`: `else if best.Compare(p) == 0 { ps = append(ps, p) }` | source |
| 12 | Talos **re-advertises routes learned from one neighbor to its other neighbors** (a Talos node is a transit speaker) | No import/export policy hook exists in `internal/bgp/bgp.go` `BuildPeer()` — no `RouteServerClient`, no policy objects; GoBGP is a full speaker; docs' troubleshooting row "An imported route is absent from the fabric → Confirm the route **was learned from a neighbor**" treats learned routes as the currency | **inferred** (high confidence) — not stated verbatim in docs |
| 13 | Unnumbered peers are resolved from the kernel neighbor table and require **exactly one** non-self IPv6 link-local neighbor carrying the `NTF_ROUTER` flag; the controller watches `RTMGRP_NEIGH` for it | `internal/bgp/neighbor.go` `SelectLinkLocalNeighbor()` (`neighbor.Flags & unix.NTF_ROUTER == 0 → skip`, `len(candidates) != 1 → fail`); `internal/app/machined/pkg/controllers/network/bgp.go` `watch.NewRtNetlink(..., unix.RTMGRP_NEIGH)` | source |
| 14 | …so the **far end must send Router Advertisements**; Talos itself has no RA sender — the upstream test fabric peer sends RAs with `ndp` precisely so nodes can discover it | `cmd/talosctl/cmd/mgmt/bgp_launch.go` `fabricSendRAs()` with comment "periodically emits IPv6 Router Advertisements on the named interface (router-lifetime 0: presence / link-local discovery only, not a default gateway) so unnumbered peers learn the host's link-local"; no equivalent config surface in `pkg/machinery/config/types/network/link.go` | source |
| 15 | BFD is configurable per neighbor (tx interval, rx interval, detect multiplier; empty block = defaults) and is **rejected on VRF-bound instances** | docs; `Validate()` → `validateBGPNeighborBehavior()`: "bfd is not supported for VRF-bound BGP instances"; `BuildPeer()` maps to `gobgpapi.BfdPeerConfig` | docs, source |
| 16 | `passive` maps to GoBGP `Transport.PassiveMode`; every peer is additionally **bound to its VRF name or its link** | `BuildPeer()` (`PassiveMode`, `BindInterface`); `internal/bgp/neighbor.go` `ResolvePeers()`: `if config.VRF != "" { BindInterface = config.VRF } else if peer.Link != "" { BindInterface = peer.Link }` | source |
| 17 | Per-neighbor `localASN` is FRR-style `local-as … replace-as`: it sets `LocalAsn` **and `ReplacePeerAsn = true`** | `BuildPeer()`: `result.Conf.LocalAsn = peer.Config.LocalASN; result.Conf.ReplacePeerAsn = true` | source |
| 18 | `holdTime` also sets keepalive to `holdTime / 3` | `BuildPeer()`: `KeepaliveInterval: hold / 3` | source |
| 19 | `importRoutes` rules: at least one prefix, **each prefix masked**, prefixes must not overlap each other or other selectors in the target instance, each source instance may appear **once**, self-import forbidden | `pkg/machinery/config/types/network/bgp.go` `validateBGPImportRoutes()` / `validateBGPImportPrefixes()` / `prefixesOverlap()` | source |
| 20 | Only unicast IPv4 + IPv6 address families are configured per peer | `BuildPeer()` builds exactly `afiSafi(Family_AFI_IP)` and `afiSafi(Family_AFI_IP6)`; `BuildImportedPath()` errors on any family other than `RF_IPv4_UC`/`RF_IPv6_UC` | source |
| 21 | Peer state is published as `BGPPeerStatus` with instance-qualified IDs (`fabric/<addr>`, `link/<iface>`), per-family received/accepted/advertised counters, router-ID, uptime, BFD state | docs "Inspect BGP state"; `internal/bgp/bgp.go` `PeerStatus()` | docs, source |
| 22 | Talos ships a **test fabric speaker** for exactly this topology: `talosctl cluster create --with-bgp` and `--with-bgp-clos` | flag names in `cmd/talosctl/cmd/mgmt/cluster/create/cmd_dev.go` (`bgpFlag = "with-bgp"`, `bgpCLOSFlag = "with-bgp-clos"`); `pkg/provision/providers/vm/bgp.go` | source |
| 23 | In CLOS mode nodes are **L3-only**: "nodes have no net0, so they are reachable only via BGP. Peer unnumbered over every [fabric uplink]", only routable address is a `/32` on `lo`, and the host runs a `--bgp-zebra` role installing learned prefixes into the host FIB | `pkg/provision/providers/vm/bgp.go` (`args = append(args, "--bgp-unnumbered", "--bgp-zebra")`); `cmd/talosctl/cmd/mgmt/bgp_launch_linux.go` `fabricZebra()` / `fabricReconcileRoutes()`; `internal/integration/api/bgp.go` `TestBGPCLOS()` docstring | source, test |
| 24 | **Distinct ASNs are required** so a fabric speaker can re-advertise one node's routes to another without the AS_PATH loop check rejecting them | `cmd/talosctl/cmd/mgmt/bgp_launch.go` comment: "has its own ASN (distinct ASNs are required so the fabric peer can re-advertise one node's routes to another without the AS_PATH loop check rejecting them), and the fabric must accept all of them" | source |
| 25 | Anycast control-plane HA pattern: **every control-plane node carries the same k8s-API VIP `/32` on `lo`**, the fabric ECMPs across the CPs, and a worker reaches the API purely over BGP | `internal/integration/api/bgp.go` `TestBGPCLOS()`: "anycast control-plane HA: every control-plane node advertises a shared k8s-API VIP /32, so the fabric ECMPs across the CPs and a non-control-plane node reaches the API purely over BGP"; `discoverCLOSVIP()` reads the global IPv4 on `lo` that is not the node's own identity; `bgp_launch_linux.go` `fabricReconcileRoutes()`: "A destination advertised by multiple nodes (e.g. an anycast control-plane VIP shared by every CP) yields multiple best paths … installed as an ECMP multipath route" | source, test |
| 26 | Talos' own VIP is a different, L2-only mechanism: `Layer2VIPConfig`, single owner elected through etcd, requires a shared L2 segment, ~up-to-1-minute failover after hard failure, must not be the `talosconfig` endpoint | docs `talos/v1.14/networking/advanced/vip` ("Requirements", "Caveats", "VIP failover behavior") | docs |
| 27 | Older Talos feature request "VIP over BGP like ARP" was closed as not planned (predates 1.14) | `siderolabs/talos#4334`, closed 2024-07-10 | issue |
| 28 | **Cilium's BGP Control Plane does not program the datapath and must not be used for in-cluster reachability** | docs `docs.cilium.io/latest/network/bgp-control-plane/bgp-control-plane/` ("Because BGP Control Plane does not program the datapath, do not use it to establish reachability within the cluster"); repo `pkg/bgp/README.md` same wording | docs |
| 29 | **Cilium will not install received BGP routes into the kernel** — the feature was requested and declined | CFP `cilium/cilium#31091` "Support injecting received BGP routes into the kernel routing table", **closed (completed→declined) 2024-03-04** by the BGP maintainer, who wrote "Importing routes from BGP is a huge change and overkill for that use case" and proposed `auto-direct-node-routes` improvements / tunnel mode instead; corroborated by `#31283` ("bgpControlPlane seems not able to inject received route into kernel FIB") and `#32826` ("accepts BGP Paths that have no effect … they are simply ignored") | issue |
| 30 | Cilium **native routing** requires the underlay to forward PodCIDRs *and* each node's kernel to know remote pod prefixes; `auto-direct-node-routes` only applies when all nodes share **one L2 segment**, otherwise "an additional system component such as a BGP daemon must be run to distribute the routes" | docs `network/concepts/routing/` "Native-Routing → Requirements on the network", items 1 and 2 | docs |
| 31 | Helm keys and semantics for that mode: `routingMode` (values `""`/native/tunnel, default **tunnel**), `ipv4NativeRoutingCIDR` (default `""`; its own description says "the user must configure the routes to reach pods, either manually or by setting the auto-direct-node-routes flag"), `nativeRoutingCIDRFromClusterPool`, `autoDirectNodeRoutes` (default **false**, "if worker nodes share a common L2 network segment"), `directRoutingSkipUnreachable` ("skipping … if the worker nodes are in a **different** L2 network segment"), `MTU` (applies to `cilium_host`/`cilium_net`/`lxc_health`, **not** to host NICs), `ipam.mode` (chart default **cluster-pool**) | `docs.cilium.io/en/stable/helm-reference/` (retrieved full table, searched for each key) | docs |
| 32 | `directRoutingSkipUnreachable` exists because of the #31091/#31124 line of complaints about `auto-direct-node-routes` erroring on multi-subnet clusters | `cilium/cilium#31124` (CFP: don't exit when a node is outside the subnet) and merged PR `cilium/cilium#32733` ("Add support for skipping direct routes on different L2 networks") | issue, docs |
| 33 | Talos-side prerequisites for running Cilium: `ipam.mode: kubernetes`; do not let Cilium mount cgroupv2/bpffs; **drop `SYS_MODULE`** from the agent capabilities; KubePrism on 7445; on 1.14+ disable the managed CNI with `kind: KubeFlannelCNIConfig / $patch: delete` (and `kind: KubeProxyConfig / enabled: false` for KPR); ~10-minute bootstrap window before node retries; `cilium connectivity test` needs a privileged PodSecurity namespace | docs `kubernetes-guides/cni/deploying-cilium` ("Prerequisites", "Machine configuration prerequisites", "Known issues") | docs |
| 34 | Documented Talos+Cilium conflict: `forwardKubeDNSToHost=true` (Talos default since 1.8) with `bpf.masquerade=true` can break CoreDNS → set `forwardKubeDNSToHost=false`; Cilium's own Talos page instead prescribes `bpf.hostLegacyRouting: true` | docs `kubernetes-guides/cni/deploying-cilium` "Known issues" (links `siderolabs/talos#9200`); docs `docs.cilium.io/stable/installation/k8s-install-talos-linux/` | docs |
| 35 | kubelet node-IP selection is configurable and, if unspecified, is derived from the **cluster podCIDRs** — a real hazard with mesh NICs + an uplink bond on the same node | `pkg/machinery/config/types/v1alpha1/v1alpha1_types.go` `KubeletNodeIPConfig`: "If not specified, node IP is picked based on cluster podCIDRs: IPv4/IPv6 address or both." | source |
| 36 | Talos cluster-level pod/service subnet fields are `cluster.network.podSubnets` and `cluster.network.serviceSubnets` | same file, `ClusterNetworkConfig` (`PodSubnet []string yaml:"podSubnets"`, `ServiceSubnet []string yaml:"serviceSubnets"`) | source |
| 37 | Talos network documents needed for the physical layout: `BondConfig` (`links`, `bondMode: 802.3ad`, `xmitHashPolicy: layer3+4`, `miimon`, `updelay`, `downdelay`, `minLinks`, embedded `CommonLinkConfig` for addresses/MTU), `VLANConfig` (`parent`, `vlanID`, name convention `parent.VID`), `DummyConfig`, `VethConfig`, `VRFConfig`, `LinkConfig` (`lo` with a `/32` works: the integration test uses exactly that) | `pkg/machinery/config/types/network/{bond,vlan,link,veth,vrf,dummy}.go` at `v1.14.4`; `internal/integration/api/bgp.go` `lo := network.NewLinkConfigV1Alpha1("lo")` with the comment "the controller advertises it and filters the 127/8 + ::1 loopback addresses — no dummy interface needed" | source, test |
| 38 | RouterOS MLAG pattern: the **client** runs a plain LACP bond; each MLAG member runs a bond with a **matching `mlag-id`** in the MLAG bridge; ICCP runs untagged over `mlag-peer-port` | `help.mikrotik.com/docs/spaces/ROS/pages/67633179/Multi-chassis+Link+Aggregation+Group` (search snippets, not read in full) | docs (snippet) |
| 39 | CRS310-8G+2S+IN = **8×2.5G RJ45 + 2×SFP+**, RouterOS v7 **license level 5**; RouterOS licensing does not gate BGP ("If a feature is not listed, it is not restricted") | `manual.mikrotik.com/hardware/crs310-8g-plus-2s-plus-in/`, `mikrotik.com/product/crs310_8g_2s_in`, `manual.mikrotik.com/docs/getting-started/routeros-licensing/mikrotik-hardware/` (licensing page read in full) | docs |
| 40 | The router side has the policy tools Talos lacks (`as-path prepend`, prefix-lists, local-pref) — pfSense FRR BGP neighbor/prefix configuration and OPNsense `os-frr` | `docs.netgate.com/pfsense/en/latest/packages/frr/bgp/config-neighbor.html` and `.../configuration.html`; `docs.opnsense.org/manual/dynamic_routing.html` (search snippets) | docs (snippet) |
| 41 | The Sidero overview states `advertise` "originates `/32` (IPv4) or `/128` (IPv6) routes for addresses on links" and "does not originate connected subnets" — **this conflicts with #4** (masked prefix, source + integration test) | docs `talos/v1.14/networking/bgp/overview` re-read 2026-10-24 vs #4 | docs (conflict, unresolved) |
| 42 | The overview lists exactly the limits in #3, #13, #15, #19; it contains **no HA/failover guidance** and no health-check mechanism tying an advertised `lo` address to a service | same page | docs |
| 43 | RouterOS 7 unnumbered-BGP interoperability tests were marked failing on 7.14 and 7.18.2; no CRS310-specific evidence found. Not relied on: the router sessions are Cilium ↔ Victim/Demiurg, numbered | `release.netlab.tools/_html/routeros7-libvirt-bgp.html`, `tests.netlab.tools/_html/routeros7-libvirt-bgp.html` (search snippets, test harness results) | docs (snippet) |

---

## 3. What the evidence establishes for the MAGI failover behavior

The mesh has no router in its path, so path selection is automatic:

| Step | Established by |
|---|---|
| A peer's prefixes arrive directly with a 1-hop AS-path and via the third node with 2 hops; the direct DAC path wins while up | #10 (AS-path length dominates) |
| A dead next-hop is **not** installed, so the kernel holds no stale pod route | #7 (`learnedRoutes()` skips invalid/unspecified next-hops) |
| With the direct link down, the pair reaches each other **through the third node** only if that node re-advertises what it learned | #12 — **inferred, not yet proven**; requires distinct ASNs (#24) |
| There are no ties to resolve: no talks-to-router path exists in the mesh instance, so the lack of prepend/local-pref (#12, `BuildPeer()`) does not matter | #10, design in `network/talos-bgp.md` |
| A node never hairpins through a peer to reach a prefix it hosts itself: local > learned, and Talos never installs its own prefixes | #10, #7 |
| Pod CIDRs reach the mesh only via Cilium → VRF `workload` instance → `importRoutes` (Cilium neither installs received routes nor can Talos originate a non-link prefix, unless #4 holds) | #28, #29, #19, #4 |
| LB IPs reach Victim/Demiurg via Cilium's BGP Control Plane, whose advertisement is selected per peer; routers are filtered on their side because Talos/Cilium offer no inbound policy | #28, #40 |

---

## 4. Strongest single sources, if you only read three things

1. **`internal/app/machined/pkg/controllers/network/internal/bgp/instance.go`** (`learnedRoutes()`) — proves exactly which routes enter the kernel, which are excluded, and that ECMP sets are built from every `Best` path.
2. **`internal/integration/api/bgp.go`** (`assertConnectedPrefixOrigination`, `assertLearnedRoute`, `TestBGPCLOS`) — upstream's own assertions about masked prefixes, learned routes, per-link next-hop counts and the anycast VIP.
3. **`cilium/cilium#31091`** — the maintainer's rejection of injecting received BGP routes, which is why the route distributor must be Talos (or FRR), not Cilium.

---

## 5. Not established — open items for MAGI

| Open item | Why it is still open |
|---|---|
| Talos re-advertising learned routes inside one instance (#12) | Inferred from `BuildPeer()` having no policy hooks + GoBGP semantics; **no Talos test exercises Talos→Talos transit without a third speaker**. Worth proving on the lab: advertise a dummy prefix on Balthasar, confirm Casper installs it when peering only via Melchior. |
| Whether tie-breaking without `multipath: true` is stable | No router-ID tie-break exists (#10); which path GoBGP keeps when `Compare() == 0` was not verified. |
| GoBGP version drift | `Compare()`/`Select()` were read from gobgp **master**; Talos pins **v4.8.0**. Re-check against the `v4.8.0` tag before relying on tie-break details. |
| Victim's OS | OPNsense 26.7.7 on both routers; `os-frr` plugin installed on Victim (not running), absent on Demiurg. |
| `advertise`: masked prefix or `/32` | #4 (source + test) contradicts #41 (docs). Decides whether PodCIDRs can be originated without the Cilium `workload` path. Verify on the lab. |
| Cold boot with all uplinks unplugged | etcd and kubelet use `lo` `/32`s that only BGP provides; never exercised. |
| Does kubelet keep working through KubePrism when isolated | Not verified; the control-plane VIP is an L2 VIP on VLAN 10 and is unavailable when the switches are down. |
| TCP/179 coexistence | Talos GoBGP and the Cilium agent both run BGP in the host namespace; whether they collide on 179 was not checked (Cilium does not listen by default). |
| IPv6 pod/LB prefixes over numbered IPv4 mesh sessions | Talos builds both address families per peer (#20); next-hop handling for IPv6 over an IPv4 session was not verified. |
| Health-aware advertisement | #42: nothing withdraws a node's advertisement when a service on it fails; only link, session or node loss does. |
| `ipam.mode: cluster-pool` vs the Talos guide's `kubernetes` prerequisite | Guide mandates `kubernetes`; chart default is `cluster-pool` (#31). Compatibility of cluster-pool with Talos 1.14 was not verified. |
| Cilium version to pair with Talos 1.14 | Sidero's guide renders 1.18.0; Cilium's current release is 1.20.4; Talos 1.14 supports Kubernetes 1.33–1.37. No compatibility matrix was found. |
| `network/diagram.md` / `network/ipam.md` accuracy | Both are snapshots of NetBox, which is the source of truth. The switch port map (routers on `ether1`/`ether2`, WAN on `ether3`, Mausoleum on `ether4`, MAGI nodes on `ether5`–`ether7`) comes from the live Aura/Mare config and from NetBox, where the ports are named `portN`. |

---

## 6. Source inventory

### Talos documentation (read in full)
- `https://docs.siderolabs.com/talos/v1.14/networking/bgp/overview`
- `https://docs.siderolabs.com/talos/v1.14/reference/configuration/network/bgpinstanceconfig`
- `https://docs.siderolabs.com/talos/v1.14/networking/bgp/cilium`
- `https://docs.siderolabs.com/talos/v1.14/networking/advanced/vip`
- `https://docs.siderolabs.com/kubernetes-guides/cni/deploying-cilium`

### Talos source at tag `v1.14.4`
- `internal/app/machined/pkg/controllers/network/bgp.go`
- `internal/app/machined/pkg/controllers/network/internal/bgp/bgp.go` (`BuildPeer`, `BuildOriginatedPath`, `BuildImportedPath`, `RouteSpec`, `PeerStatus`, `ServerKey`, `PeerKey`)
- `internal/app/machined/pkg/controllers/network/internal/bgp/instance.go` (`EnsureServer`, `ReconcileOriginated`, `Snapshot`, `learnedRoutes`)
- `internal/app/machined/pkg/controllers/network/internal/bgp/neighbor.go` (`ResolvePeers`, `SelectLinkLocalNeighbor`)
- `internal/app/machined/pkg/controllers/network/internal/bgp/runtime.go` (`advertisedAddresses`, `connectedPrefixes`, `routerID`, `validateLinkDomain`, `VRFIndex`)
- `internal/app/machined/pkg/controllers/network/internal/bgp/imports.go` (`ReconcileImportedPaths`)
- `pkg/machinery/config/types/network/bgp.go` (`Validate`, `validateBGPImportRoutes`, `validateBGPNeighborBehavior`)
- `pkg/machinery/config/types/network/{bond,vlan,link,veth,vrf,dummy}.go`
- `pkg/machinery/config/types/v1alpha1/v1alpha1_types.go` (`KubeletNodeIPConfig`, `ClusterNetworkConfig`)
- `internal/integration/api/bgp.go`, `internal/integration/api/bgp_workload.go`
- `cmd/talosctl/cmd/mgmt/bgp_launch.go`, `cmd/talosctl/cmd/mgmt/bgp_launch_linux.go`
- `cmd/talosctl/cmd/mgmt/cluster/create/cmd_dev.go`
- `pkg/provision/providers/vm/bgp.go`
- `go.mod` (`github.com/osrg/gobgp/v4 v4.8.0`)

### GoBGP (master at time of reading; Talos pins v4.8.0)
- `internal/pkg/table/path.go` → `(*Path).Compare()`
- `internal/pkg/table/destination.go` → `Select(...DestinationSelectOption{MultiPath:true})`, `getMultiBestPath`
- `pkg/server/server.go` → `notifyBestWatcher`, `dstsToPaths`

### GitHub records
- `siderolabs/talos` commit `2193b5781` — "feat: native BGP support via embedded GoBGP"
- `siderolabs/talos` releases `v1.14.0-beta.0`, `v1.14.0-rc.1/2`, `v1.14.0`, `v1.14.4`, `v1.13.12`
- `siderolabs/talos#11126` — Native GoBGP Integration for BGP Configuration via MachineConfig (design rationale, RR/VIP discussion)
- `talos-systems/talos#4334` — "Support VIP over BGP like ARP" (closed, not planned)
- `cilium/cilium#31091` — CFP: inject received BGP routes (closed, declined)
- `cilium/cilium#31283` — BGP route not injected into kernel FIB
- `cilium/cilium#32826` — BGP control plane accepts paths that have no effect
- `cilium/cilium#31124` / merged PR `cilium/cilium#32733` — `direct-routing-skip-unreachable`

### Cilium documentation
- `https://docs.cilium.io/en/stable/network/concepts/routing/` (Encapsulation vs Native-Routing)
- `https://docs.cilium.io/en/latest/network/bgp-control-plane/bgp-control-plane/` and `.../bgp-control-plane-configuration/` (PodCIDR / Service advertisement types, `transport.sourceInterface`)
- `https://docs.cilium.io/en/stable/helm-reference/`
- `https://docs.cilium.io/en/stable/installation/k8s-install-talos-linux/`
- `https://github.com/cilium/cilium/blob/main/pkg/bgp/README.md`

### Network-vendor documentation
- MikroTik MLAG: `https://help.mikrotik.com/docs/spaces/ROS/pages/67633179/Multi-chassis+Link+Aggregation+Group`
- MikroTik licensing: `https://manual.mikrotik.com/docs/getting-started/routeros-licensing/mikrotik-hardware/`
- MikroTik CRS310 hardware: `https://manual.mikrotik.com/hardware/crs310-8g-plus-2s-plus-in/`
- pfSense FRR BGP: `https://docs.netgate.com/pfsense/en/latest/packages/frr/bgp/` (configuration, neighbor, example)
- OPNsense FRR: `https://docs.opnsense.org/manual/dynamic_routing.html`

---

## 7. Related notes in this repo

- [`network/talos-bgp.md`](../../network/talos-bgp.md) — the design write-up (two BGP planes, addressing/ASN plan, per-node Talos config, Cilium resources, router filter spec, failure matrix, validation plan).
- [`network/diagram.md`](../../network/diagram.md), [`network/ipam.md`](../../network/ipam.md) — NetBox snapshots.
