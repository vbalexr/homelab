# Router side of the MAGI BGP peering

What the two routers (Victim, Demiurg) need so the cluster's LoadBalancer addresses are reachable. The node and Cilium side is in
`cluster/magi/talos/config/` and `overlays/magi/infrastructure/cilium/`; the overview is in [README.md](README.md).

## Policy

Both routers peer with every node's **static uplink** address (lab VLAN), never over the CARP VIP.

- Accept only exact host routes inside the LoadBalancer pools: `/32` within `10.1.1.0/24` and `/128` within `fd7a:2201:1ac::/64`.
  Reject everything else, including covering prefixes and pod CIDRs.
- Advertise nothing to the nodes.
- Configure the sessions on **both** routers and keep them established regardless of CARP master/backup state; the FRR configuration
  is not synced between them. Each node peers with both, so losing one router leaves every address reachable.
- Multipath/ECMP on, so an address advertised by several nodes is load-balanced.

## OPNsense (`os-frr`) configuration

AS 65000 on both routers, 12 sessions in total (6 neighbors per router: 3 nodes x IPv4 and IPv6), keepalive 3 s / hold 9 s. Configure
each router in the GUI, the same on both except the router ID (the router's own lab address).

**Routing > General**
- Enable routing; turn **off** "Enable CARP Failover" so the sessions stay up on both routers.
- Turn **off** the automatic firewall rules: the "Pass BGP" rule they create opens TCP/179 on every interface, WAN included.

**Routing > BGP > General**: enable, AS `65000`, router ID, maximum paths `3`, log neighbor changes.

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

**Neighbors** (all remote AS `65021`, keepalive `3`, hold `9`, route map out `MAGI-OUT`)

| Address | Node | Route map in | Update source |
|---|---|---|---|
| `10.1.0.11` / `.12` / `.13` | Balthasar / Casper / Melchior | `MAGI-IN4` | lab interface |
| `fd7a:2201:1ab::11` / `::12` / `::13` | Balthasar / Casper / Melchior | `MAGI-IN6` | none |

Equivalent FRR:

```
ip prefix-list MAGI-LB4 seq 10 permit 10.1.1.0/24 ge 32 le 32
ipv6 prefix-list MAGI-LB6 seq 10 permit fd7a:2201:1ac::/64 ge 128 le 128
route-map MAGI-IN4 permit 10
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
  neighbor MAGI route-map MAGI-IN4 in
  neighbor MAGI route-map MAGI-OUT out
  maximum-paths 3
```

The IPv6 family is the same over the IPv6 neighbors with `MAGI-IN6`.

## Checks

- `vtysh -c 'show bgp summary'` on each router: six established sessions. `cilium-cli bgp peers` on the cluster.
- Each node pins the source address of its router sessions (`CiliumBGPNodeConfigOverride`): the uplink also holds a SLAAC address that
  the kernel prefers, and the routers only accept the static ones.

## Notes

- Client VLANs reach the pool only where a firewall rule allows it. The trusted VLAN passes all protocols to both pool prefixes (one
  rule per family, on both routers); other VLANs need their own rule for what they use.
- Hosts on the lab VLAN that are not cluster nodes must not use pool addresses: the nodes reply directly, the routers see one
  direction of the flow, and the connection stalls after about 64 KB.
- If a router has no IPv4 default route, `pkg install os-frr` cannot reach the repository from it: create the packages on the other
  router (`pkg create -o <dir> os-frr frr10 frr10-pythontools c-ares libssh libunwind libyang3 net-snmp protobuf-c protobuf abseil
  jsoncpp rtrlib`), copy them over `ssh` and `pkg add -f` them in dependency order, then `service configd start`.
- Not configured yet: a Null0/blackhole route for the two pool prefixes.

## Failure behavior

| Failure | Effect |
|---|---|
| One router down | The other still holds a session to every node; ingress continues. |
| Both routers down | External LoadBalancer reachability is lost; the cluster is unaffected. |
| One node's uplink down | Its sessions drop and the routers withdraw its addresses; its pods stay reachable through the mesh. |
| One DAC link down | The 9 s hold timer drops the session; the pair reaches each other via the third node. |
| Both DAC links of one node down | Static routes over the uplink carry pod, etcd and kubelet traffic until the mesh returns. |
| One node down | Peers withdraw its prefixes; pods reschedule; etcd keeps quorum (2 of 3). |
