# IPAM

Source of truth: NetBox. Snapshot only — regenerate when addressing changes.

---

## Site: Origin

Flat network, no VLAN segmentation.

### Prefix `10.1.92.0/24` — Origin site LAN

| IP            | Device / VM                | Interface | Description                  |
|---------------|-----------------------------|-----------|-------------------------------|
| 10.1.92.1     | USG-3P                     | eth0      | USG-3P gateway                |
| 10.1.92.2     | USW Flex Mini               | eth0      | USW Flex Mini                 |
| 10.1.92.3     | U6-Lite — Living Room       | eth0      | AP - Living Room              |
| 10.1.92.4     | U6-Lite — Farm              | eth0      | AP - Farm                     |
| 10.1.92.10    | ThinkCentre M720q           | eth0      | Proxmox host (M720q)          |
| 10.1.92.11    | UniFi OS Controller (LXC)   | eth0      | LXC - UniFi OS Controller     |
| 10.1.92.12    | Technitium (LXC)            | eth0      | LXC - Technitium (DNS)        |

Devices with no tracked IP: **ONU** (Fiber ONU).

---

## Site: Home

VLAN-segmented. Gateways are **Victim** and **Demiurg**, a CARP pair (both active, pfsync between them); CARP VIPs
provide failover addressing on each VLAN.

### VLAN 500 — Mgmt (`10.0.0.0/24`, `fd7a:2201:5e72::/64`)
Network infra (switches, APs).

| IP                     | Device                | Interface   | Description                          |
|------------------------|------------------------|-------------|----------------------------------------|
| 10.0.0.1                | (CARP VIP)             | vlan0.500   | Mgmt VLAN gateway VIP                  |
| fd7a:2201:5e72::1       | (CARP VIP)             | vlan0.500   | Mgmt VLAN gateway VIP                  |
| 10.0.0.2                | Victim                 | vlan0.500   | Mgmt VLAN gateway                      |
| 10.0.0.3                 | Demiurg                | vlan0.500   | Mgmt VLAN gateway |
| fd7a:2201:5e72::2       | Victim                 | vlan0.500   | Mgmt VLAN gateway                      |
| fd7a:2201:5e72::3        | Demiurg                | vlan0.500   | Mgmt VLAN gateway |
| 10.0.0.5                | Aura                   | mgmt0       | Switch management                      |
| fd7a:2201:5e72::5       | Aura                   | mgmt0       | Switch management                      |
| 10.0.0.6                | Mare                   | mgmt0       | Switch management                      |
| fd7a:2201:5e72::6       | Mare                   | mgmt0       | Switch management                      |
| 10.0.0.8                | Sebas Tian              | mgmt0       | Switch management                      |
| 10.0.0.9                | Tsuare                  | mgmt0       | AP management                          |
| fd7a:2201:5e72::9       | Tsuare                  | mgmt0       | AP management                          |
| 10.0.0.10               | Pestonya                | mgmt0       | AP management                          |
| fd7a:2201:5e72::10      | Pestonya                | mgmt0       | AP management                          |
| 10.0.0.100              | BeQuiet                | eth0        | Workstation (Sebas Tian ether12, untagged Mgmt VLAN 500) |
| fd7a:2201:5e72::100     | BeQuiet                | eth0        | Workstation (Sebas Tian ether12, untagged Mgmt VLAN 500) |
| 10.0.0.15               | Mausoleum                | kvm0        | KVM module management port (patch panel 8, Sebas Tian ether7, untagged Mgmt VLAN 500) |

### VLAN 10 — Lab (`10.1.0.0/24`, `fd7a:2201:1ab::/64`)
Cluster frontend, VMs, node management.

| IP                     | Device                     | Interface   | Description                                |
|------------------------|------------------------------|-------------|----------------------------------------------|
| 10.1.0.1                | (CARP VIP)                  | vlan0.10    | Lab VLAN gateway VIP                          |
| fd7a:2201:1ab::1        | (CARP VIP)                  | vlan0.10    | Lab VLAN gateway VIP                          |
| 10.1.0.2                | Victim                      | vlan0.10    | Lab VLAN gateway                              |
| 10.1.0.3                 | Demiurg                | vlan0.10    | Lab VLAN gateway |
| fd7a:2201:1ab::2        | Victim                      | vlan0.10    | Lab VLAN gateway                              |
| fd7a:2201:1ab::3         | Demiurg                | vlan0.10    | Lab VLAN gateway |
| 10.1.0.10               | (VIP, one MAGI node)        | bond-up     | MAGI Talos HA control plane VIP (Talos `Layer2VIPConfig`, IPv4 only, held by one node at a time) — `magi.vbalex.com` |
| 10.1.0.15               | Mausoleum                   | bond0 (eth0+eth1)   | LAN uplink — bond0 (eth0+eth1, LACP) into the MLAG pair on Mare `ether4` + Aura `ether4`, tagged VLAN 10. VLAN 10 and the WAN-facing path run standard MTU 1500 — no jumbo frames end-to-end. |
| fd7a:2201:1ab::15       | Mausoleum                   | bond0 (eth0+eth1)   | LAN uplink — bond0 (eth0+eth1, LACP) into the MLAG pair on Mare `ether4` + Aura `ether4`, tagged VLAN 10. VLAN 10 and the WAN-facing path run standard MTU 1500 — no jumbo frames end-to-end. |
| 10.1.0.20               | NUT Server                  | eth0        | NUT server (Sebas Tian ether14, untagged)     |
| fd7a:2201:1ab::20       | NUT Server                  | eth0        | NUT server (Sebas Tian ether14, untagged)     |
| 10.1.0.11               | Balthasar                   | bond-up  | MAGI Talos node uplink (LACP bond `enp5s0`+`enp4s0`, untagged VLAN 10, static, default route via 10.1.0.1) |
| fd7a:2201:1ab::11               | Balthasar                   | bond-up  | MAGI Talos node uplink (LACP bond `enp5s0`+`enp4s0`, untagged VLAN 10, static, default route via 10.1.0.1) |
| 10.1.0.12               | Casper                      | bond-up  | MAGI Talos node uplink (LACP bond `enp5s0`+`enp4s0`, untagged VLAN 10, static, default route via 10.1.0.1) |
| fd7a:2201:1ab::12               | Casper                      | bond-up  | MAGI Talos node uplink (LACP bond `enp5s0`+`enp4s0`, untagged VLAN 10, static, default route via 10.1.0.1) |
| 10.1.0.13               | Melchior                    | bond-up  | MAGI Talos node uplink (LACP bond `enp5s0`+`enp4s0`, untagged VLAN 10, static, default route via 10.1.0.1) |
| fd7a:2201:1ab::13               | Melchior                    | bond-up  | MAGI Talos node uplink (LACP bond `enp5s0`+`enp4s0`, untagged VLAN 10, static, default route via 10.1.0.1) |


### VLAN 15 — Home (`172.16.15.0/24`, `fd7a:2201:ca5a::/64`)
Trusted clients.

| IP                       | Device       | Interface   | Description                    |
|--------------------------|---------------|-------------|----------------------------------|
| 172.16.15.1               | (CARP VIP)    | vlan0.15    | Home VLAN gateway VIP            |
| fd7a:2201:ca5a::1         | (CARP VIP)    | vlan0.15    | Home VLAN gateway VIP            |
| 172.16.15.2               | Victim        | vlan0.15    | Home VLAN gateway                |
| 172.16.15.3              | Demiurg                | vlan0.15    | Home VLAN gateway |
| fd7a:2201:ca5a::2         | Victim        | vlan0.15    | Home VLAN gateway                |
| fd7a:2201:ca5a::3        | Demiurg                | vlan0.15    | Home VLAN gateway |

### VLAN 20 — IoT (`172.16.20.0/24`)
Untrusted smart devices.

| IP             | Device    | Interface   | Description                |
|-----------------|------------|-------------|-------------------------------|
| 172.16.20.1      | (CARP VIP) | vlan0.20    | IoT VLAN gateway VIP          |
| 172.16.20.2      | Victim     | vlan0.20    | IoT VLAN gateway   |
| 172.16.20.3              | Demiurg                | vlan0.20    | IoT VLAN gateway |
| 172.16.20.10     | Zigbee Bridge | eth0     | Zigbee Bridge (untagged, via Sebas Tian ether2 / patch panel front3) |

### VLAN 25 — CCTV (`172.16.25.0/24`)
Camera VLAN.

| IP             | Device                          | Interface | Description                       |
|-----------------|-----------------------------------|-----------|--------------------------------------|
| 172.16.25.1      | (CARP VIP)                        | vlan0.25  | CCTV VLAN gateway VIP                |
| 172.16.25.2      | Victim                            | vlan0.25  | CCTV VLAN gateway         |
| 172.16.25.3       | Demiurg                           | vlan0.25  | CCTV VLAN gateway         |
| 172.16.25.200    | Frigate                           | eth0      | Frigate NVR (other rack, not tracked; uplink to Sebas Tian ether9, untagged) |

### VLAN 2 / VLAN 3 — WAN transit (L2 only)

The fiber ONT (patch panel 1, Aura `ether3`, VLAN 2) and the Starlink terminal (patch panel 2, Mare `ether3`, VLAN 3) are carried across
the MLAG pair to both Victim and Demiurg. No addresses are tracked here; the ISP-facing addressing lives
on the routers.

### Routed, no L2 — MAGI cluster mesh (`10.1.3.0/28`, `fd7a:2201:1ad::/64`)

Direct DAC links between the MAGI Talos nodes, MTU 1500, carrying the cluster BGP mesh. No switch,
VLAN or gateway. Links are point-to-point (`/31` and `/127`); node identities are `/32` and `/128` on `lo`.
See [talos-bgp.md](talos-bgp.md). Each node's `bond-up` also holds metric-2000 static routes to the other two
nodes' `lo` addresses and PodCIDRs via their uplink addresses; they are used only when no mesh path exists.

| Prefix / IP                         | Device                                 | Interface       | Description |
|-------------------------------------|----------------------------------------|-----------------|-------------|
| 10.1.3.0/31, fd7a:2201:1ad::/127     | Balthasar `.0`/`::` ↔ Casper `.1`/`::1` | `eno2` / `eno2` | Mesh link |
| 10.1.3.2/31, fd7a:2201:1ad::2/127    | Balthasar `.2`/`::2` ↔ Melchior `.3`/`::3` | `eno1` / `eno2` | Mesh link |
| 10.1.3.4/31, fd7a:2201:1ad::4/127    | Casper `.4`/`::4` ↔ Melchior `.5`/`::5` | `eno1` / `eno1` | Mesh link |
| 10.1.3.11/32, fd7a:2201:1ad::11/128    | Balthasar | lo | Node identity, etcd/kubelet address, advertised over BGP (ASN 65011) |
| 10.1.3.12/32, fd7a:2201:1ad::12/128   | Casper    | lo | Node identity, etcd/kubelet address, advertised over BGP (ASN 65012) |
| 10.1.3.13/32, fd7a:2201:1ad::13/128   | Melchior  | lo | Node identity, etcd/kubelet address, advertised over BGP (ASN 65013) |

### Kubernetes pod and service networks (not routed outside the cluster)

| Prefix | Use |
|--------|-----|
| 10.244.0.0/16, fd7a:2201:1ae::/56 | Pod CIDRs |
| 10.244.0.0/24, fd7a:2201:1ae::/64 | Casper pods |
| 10.244.1.0/24, fd7a:2201:1ae:1::/64 | Balthasar pods |
| 10.244.3.0/24, fd7a:2201:1ae:3::/64 | Melchior pods |
| 10.96.0.0/12, fd7a:2201:1af::/112 | Service CIDRs |

### Routed, no L2 — Kubernetes LoadBalancer pool (`10.1.1.0/24`, `fd7a:2201:1ac::/64`)

**Not a VLAN and not attached to any interface or switch port.** These prefixes exist only as
BGP-advertised `/32` / `/128` host routes originated by Cilium on the MAGI nodes and learned by
Victim and Demiurg. Addresses are allocated dynamically by Cilium LB IPAM, so individual IPs are not tracked
here.

Pinned addresses (`lbipam.cilium.io/ips` annotation on the Service):

| IP                                | Service                | Port | Description                                  |
|-----------------------------------|------------------------|------|------------------------------------------------|
| 10.1.1.10, fd7a:2201:1ac::10      | shared/shared-gateway  | 80, 443 | Gateway for the shared apps (vault, photos, netbox)|
| 10.1.1.11, fd7a:2201:1ac::11      | home/home-gateway      | 80, 443 | Gateway for the home apps                       |
| 10.1.1.12, fd7a:2201:1ac::12      | sspti/sspti-gateway    | 80, 443 | Gateway for aa-sspti                            |
| 10.1.1.13, fd7a:2201:1ac::13      | mrakaki/mrakaki-gateway | 80, 443 | Gateway for the mrakaki apps                   |
| 10.1.1.14, fd7a:2201:1ac::14      | chilu/chilu-gateway    | 80       | Gateway for chilu-bb                            |
| 10.1.1.15, fd7a:2201:1ac::15      | gameservers/minecraft  | 25565/tcp, 19132/udp, 24454/udp | Minecraft Paper (Java), Geyser (Bedrock) and voice chat (externalTrafficPolicy Local) |
| 10.1.1.20, fd7a:2201:1ac::20      | database/postgres      | 5432 | PostgreSQL (externalTrafficPolicy Local)       |
| 10.1.1.21, fd7a:2201:1ac::21      | database/mariadb       | 3306 | MariaDB (externalTrafficPolicy Local)          |
| 10.1.1.23, fd7a:2201:1ac::23      | home/mosquitto         | 1883, 9001 | Mosquitto MQTT and WebSocket (externalTrafficPolicy Local) |
| 10.1.1.30, fd7a:2201:1ac::30      | dns/technitium-0       | 53/udp, 53/tcp | Technitium DNS resolver 1 (externalTrafficPolicy Local) |
| 10.1.1.31, fd7a:2201:1ac::31      | dns/technitium-1       | 53/udp, 53/tcp | Technitium DNS resolver 2 (externalTrafficPolicy Local) |
