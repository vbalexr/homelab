# MAGI node hardware

Facts read from the nodes in Talos maintenance mode (v1.14.4, plain ISO).
Nodes: Balthasar, Casper, Melchior. Identical platform.

## Platform

| Item | Value |
|---|---|
| Machine | Qotom Q20353G9 |
| CPU | Intel Atom C3908, 16 cores / 16 threads |
| Install disk | 256 GB NVMe (OS) |
| Data disk | 1 TB NVMe (Longhorn) |
| RJ45 NICs | 5x Intel I226-V 2.5G, driver `igc` |
| SFP+ NICs | 4x Intel X553 10GbE, driver `ixgbe` |
| Extensions | `intel-ucode`, `iscsi-tools`, `util-linux-tools` (see `schematic.yaml`) |

No NIC driver extension is needed: `igc` and `ixgbe` are in the stock kernel.

## Firmware

| Setting | Value |
|---|---|
| Boot mode | UEFI |
| Secure Boot | enabled, enforced with the Sidero Labs signing key (Talos SecureBoot ISO), all three nodes |
| TPM | not available on any node (Talos: "TPM device is not available"); BIOS setting not checked |

## Network interfaces

Names are the same on all three nodes; MACs differ.

| Name | PCI | Chip | Role |
|---|---|---|---|
| `eno1` | `0000:0b:00.0` | X553 SFP+ | mesh |
| `eno2` | `0000:0b:00.1` | X553 SFP+ | mesh |
| `eno3` | `0000:0d:00.0` | X553 SFP+ | unused |
| `eno4` | `0000:0d:00.1` | X553 SFP+ | unused |
| `enp4s0` | `0000:04:00.0` | I226-V | uplink (2.5G, `bond-up` member, to Mare) |
| `enp5s0` | `0000:05:00.0` | I226-V | uplink (2.5G, `bond-up` member, to Aura) |
| `enp6s0` | `0000:06:00.0` | I226-V | unused |
| `enp7s0` | `0000:07:00.0` | I226-V | unused |
| `enp8s0` | `0000:08:00.0` | I226-V | unused |

### MACs

| Port | Balthasar | Casper | Melchior |
|---|---|---|---|
| `eno1` | `20:7c:14:f8:a3:5e` | `20:7c:14:f8:a4:00` | `20:7c:14:f8:a3:70` |
| `eno2` | `20:7c:14:f8:a3:5f` | `20:7c:14:f8:a4:01` | `20:7c:14:f8:a3:71` |
| `eno3` | `20:7c:14:f8:a3:60` | `20:7c:14:f8:a4:02` | `20:7c:14:f8:a3:72` |
| `eno4` | `20:7c:14:f8:a3:61` | `20:7c:14:f8:a4:03` | `20:7c:14:f8:a3:73` |
| `enp4s0` | `20:7c:14:f8:a3:59` | `20:7c:14:f8:a3:fb` | `20:7c:14:f8:a3:6b` |
| `enp5s0` | `20:7c:14:f8:a3:5a` | `20:7c:14:f8:a3:fc` | `20:7c:14:f8:a3:6c` |
| `enp6s0` | `20:7c:14:f8:a3:5b` | `20:7c:14:f8:a3:fd` | `20:7c:14:f8:a3:6d` |
| `enp7s0` | `20:7c:14:f8:a3:5c` | `20:7c:14:f8:a3:fe` | `20:7c:14:f8:a3:6e` |
| `enp8s0` | `20:7c:14:f8:a3:5d` | `20:7c:14:f8:a3:ff` | `20:7c:14:f8:a3:6f` |

## 10G DAC mesh

All three links negotiate 10 Gbps, full duplex.

| Link | Ends | Verified by |
|---|---|---|
| Balthasar - Casper | Balthasar `eno2` / Casper `eno2` | unplug test |
| Balthasar - Melchior | Balthasar `eno1` / Melchior `eno2` | unplug test |
| Casper - Melchior | Casper `eno1` / Melchior `eno1` | unplug test |

Each node's two mesh ports are on the same X553 controller (`0b:00.0` and `0b:00.1`).

## Disks

Select by serial. NVMe device names (`nvme0n1`, `nvme1n1`) are not stable:
they differ between nodes and can swap between boots on the same node. The
256 GB model also differs on Melchior.

| Node | Role | Size | Model | Serial | WWID |
|---|---|---|---|---|---|
| Balthasar | OS | 256 GB | XPG GAMMIX S11 Pro | `2M322LACB8FL` | `eui.0000000000000000707c1802322acb8f` |
| Balthasar | Longhorn | 1 TB | KINGSTON OM3PGP41024P-A0 | `50026B72836AD043` | `eui.00000000000000000026b72836ad0435` |
| Casper | OS | 256 GB | XPG GAMMIX S11 Pro | `2M322L1CBHHE` | `eui.0000000000000000707c1800023221cb` |
| Casper | Longhorn | 1 TB | KINGSTON OM3PGP41024P-A0 | `50026B72836AC7FD` | `eui.00000000000000000026b72836ac7fd5` |
| Melchior | OS | 256 GB | ADATA LEGEND 700 | `2N182L15CJGA` | `nvme.1cc1-324e3138324c3135434a4741-4144415441204c4547454e4420373030-00000001` |
| Melchior | Longhorn | 1 TB | KINGSTON OM3PGP41024P-A0 | `50026B72836AD7A4` | `eui.00000000000000000026b72836ad7a45` |
