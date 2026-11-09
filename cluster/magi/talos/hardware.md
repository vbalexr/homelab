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

Names are the same on all three nodes.

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

## 10G DAC mesh

All three links negotiate 10 Gbps, full duplex.

| Link | Ends | Verified by |
|---|---|---|
| Balthasar - Casper | Balthasar `eno2` / Casper `eno2` | unplug test |
| Balthasar - Melchior | Balthasar `eno1` / Melchior `eno2` | unplug test |
| Casper - Melchior | Casper `eno1` / Melchior `eno1` | unplug test |

Each node's two mesh ports are on the same X553 controller (`0b:00.0` and `0b:00.1`).

## Disks

Select by serial (`config/<node>.yml`). NVMe device names (`nvme0n1`, `nvme1n1`) are not stable:
they differ between nodes and can swap between boots on the same node. The
256 GB model also differs on Melchior.

| Node | Role | Size | Model |
|---|---|---|---|
| Balthasar | OS | 256 GB | XPG GAMMIX S11 Pro |
| Balthasar | Longhorn | 1 TB | KINGSTON OM3PGP41024P-A0 |
| Casper | OS | 256 GB | XPG GAMMIX S11 Pro |
| Casper | Longhorn | 1 TB | KINGSTON OM3PGP41024P-A0 |
| Melchior | OS | 256 GB | ADATA LEGEND 700 |
| Melchior | Longhorn | 1 TB | KINGSTON OM3PGP41024P-A0 |

The serials the selectors use are in `config/<node>.yml`.
