# MAGI Talos install

Three nodes (Balthasar, Casper, Melchior), all control plane and worker, full HA, rebuilt from the files in
this repo. Hardware facts: `hardware.md`. Image definition: `schematic.yaml`. Cilium BGP resources:
`../../../overlays/magi/infrastructure/cilium/`. Addressing: NetBox. Design and evidence: `../../../network/talos-bgp.md`,
`../README.md`.

## Images

| Item | Value |
|---|---|
| Talos | v1.14.4 |
| Schematic ID | `f9355a00f82a90f7dda4b0548c181fc96a2bdc14fd5eea61542b90562ed339ae` |
| SecureBoot ISO | `https://factory.talos.dev/image/<schematic>/v1.14.4/metal-amd64-secureboot.iso` |
| Plain ISO | `https://factory.talos.dev/image/<schematic>/v1.14.4/metal-amd64.iso` |
| Installer (config `install image`) | `factory.talos.dev/metal-installer-secureboot/<schematic>:v1.14.4` |

SecureBoot uses the Sidero Labs signing key from Image Factory.

## Build from scratch

Config lives in `config/` (`common.yaml` + `<node>.yml`); `generated/` is git-ignored and holds the secrets and
rendered configs. Run from this directory. Tools: `talosctl` 1.14.4, `kubectl`, and the Cilium CLI
(`sudo pacman -S cilium-cli kubectl`; the binary is `cilium-cli`).

### 1. Prepare the nodes

1. Boot each node from the SecureBoot ISO on USB. Enrolling the SecureBoot keys is manual: Esc at the boot
   menu, `Enroll Secure Boot keys: auto`. The node then sits in maintenance mode on a DHCP address.
2. Cable the mesh DACs (Balthasar `eno2` ↔ Casper `eno2`, Balthasar `eno1` ↔ Melchior `eno2`, Casper `eno1` ↔
   Melchior `eno1`) and each node's `enp5s0` to Aura and `enp4s0` to Mare (Balthasar `ether5`, Casper
   `ether6`, Melchior `ether7` on both switches).
3. Aura/Mare need one LACP bond per node (`bondBalthasar`/`bondCasper`/`bondMelchior`, 802.3ad, LACP active,
   rate 1sec, `layer-3-and-4`, mlag-id 201/202/203, untagged VLAN 10), identical on both switches.

### 2. Render the configs

```sh
# once: secrets, base config, talosconfig. Keep generated/secrets.yaml to rebuild the same cluster identity.
talosctl gen secrets -o generated/secrets.yaml
talosctl gen config magi https://10.1.0.10:6443 --with-secrets generated/secrets.yaml \
  --output-types controlplane,talosconfig --output-dir generated --with-docs=false --with-examples=false

# once: password of the NUT user `observer` on the NUT server 10.1.0.20 (see "UPS shutdown"), from its /etc/nut/upsd.users
ssh root@10.1.0.20 'grep -A3 "^\[observer\]" /media/root-ro/etc/nut/upsd.users' | sed -n 's/.*password = //p' | tr -d '\n' > generated/nut-password
sed "s/__NUT_PASSWORD__/$(cat generated/nut-password)/" config/nut-client.yaml > generated/nut-client.yaml

# per node (balthasar, casper, melchior)
talosctl machineconfig patch generated/controlplane.yaml \
  --patch @config/common.yaml --patch @generated/nut-client.yaml --patch @config/balthasar.yml -o generated/balthasar.yaml
talosctl validate --config generated/balthasar.yaml --mode metal

talosctl machineconfig patch generated/controlplane.yaml \
  --patch @config/common.yaml --patch @generated/nut-client.yaml --patch @config/casper.yml -o generated/casper.yaml
talosctl validate --config generated/casper.yaml --mode metal

talosctl machineconfig patch generated/controlplane.yaml \
  --patch @config/common.yaml --patch @generated/nut-client.yaml --patch @config/melchior.yml -o generated/melchior.yaml
talosctl validate --config generated/melchior.yaml --mode metal
```

### 3. Install and bootstrap

```sh
# first install, node in maintenance mode (DHCP ip); it installs to the 256 GB NVMe chosen by serial and reboots
talosctl apply-config --insecure -n <dhcp-ip> -e <dhcp-ip> -f generated/balthasar.yaml   # repeat per node

# once, after all three are installed (they sit in "booting" until this)
export TALOSCONFIG=$PWD/generated/talosconfig
talosctl bootstrap -n 10.1.0.11 -e 10.1.0.11
talosctl kubeconfig generated/kubeconfig -n 10.1.0.11 -e 10.1.0.11
talosctl -n 10.1.0.11 get bgppeerstatus          # fabric/* sessions ESTABLISHED
```

Maintenance-mode queries need `--insecure -n <ip> -e <ip>`. The nodes stay NotReady until Cilium is installed.

### 4. Cilium

Cilium 1.20.4 via the CLI: native routing, dual-stack, replaces kube-proxy, Gateway API, BGP control plane.

```sh
export KUBECONFIG=$PWD/generated/kubeconfig

# Gateway API CRDs (v1.6.3, the version Cilium 1.20 requires)
B=https://raw.githubusercontent.com/kubernetes-sigs/gateway-api/v1.6.3/config/crd/standard
for c in gatewayclasses gateways httproutes referencegrants grpcroutes backendtlspolicies tlsroutes listenersets tcproutes udproutes; do
  kubectl apply --server-side -f $B/gateway.networking.k8s.io_$c.yaml
done

cilium-cli install --version 1.20.4 \
  --set ipam.mode=kubernetes \
  --set kubeProxyReplacement=true \
  --set k8sServiceHost=localhost --set k8sServicePort=7445 \
  --set routingMode=native \
  --set ipv4.enabled=true --set ipv6.enabled=true \
  --set ipv4NativeRoutingCIDR=10.244.0.0/16 --set ipv6NativeRoutingCIDR=fd7a:2201:1ae::/56 \
  --set autoDirectNodeRoutes=false \
  --set bgpControlPlane.enabled=true \
  --set gatewayAPI.enabled=true \
  --set bpf.hostLegacyRouting=true \
  --set nodePort.directRoutingDevice=bond-up --set devices='{eno1,eno2,bond-up}' \
  --set cgroup.autoMount.enabled=false --set cgroup.hostRoot=/sys/fs/cgroup \
  --set securityContext.capabilities.ciliumAgent='{CHOWN,KILL,NET_ADMIN,NET_RAW,IPC_LOCK,SYS_ADMIN,SYS_RESOURCE,DAC_OVERRIDE,FOWNER,SETGID,SETUID}' \
  --set securityContext.capabilities.cleanCiliumState='{NET_ADMIN,SYS_ADMIN,SYS_RESOURCE}'

cilium-cli status --wait
kubectl get nodes; kubectl get gatewayclass
```

### 5. Pod routing

The veth pair, VRF and the Talos `workload` BGP instance are already in `config/`. Cilium advertises each node's
PodCIDRs to it; Talos imports them into the mesh:

```sh
# the BGP resources are applied by Flux (overlays/magi/infrastructure/cilium); to apply them by hand before Flux runs:
kubectl apply -k ../../../overlays/magi/infrastructure/cilium
cilium-cli bgp peers                              # talos-workload established, 1 prefix advertised per family
talosctl -n 10.1.0.11 get bgppeerstatus           # fabric/* and workload/* ESTABLISHED
```

Check: each node should hold 8 BGP routes (both peers' `lo` and pod CIDRs, both families), and pods on
different nodes ping each other over IPv4 and IPv6 (`kubectl run` busybox pods pinned with `nodeName`).

## Day-2 commands

```sh
# config change, one node at a time (apply modes: auto, no-reboot, staged, try; there is no "reboot" mode)
talosctl apply-config --mode auto -n 10.1.0.11 -e 10.1.0.11 -f generated/balthasar.yaml
# staged writes the config for the next boot:
talosctl apply-config --mode staged -n 10.1.0.11 -e 10.1.0.11 -f generated/balthasar.yaml
talosctl reboot -n 10.1.0.11 -e 10.1.0.11 --wait=false

# risky network change (uplink, bond): applied with automatic rollback, then confirmed with a second apply
talosctl apply-config --mode try --timeout 4m -n 10.1.0.11 -e 10.1.0.11 -f generated/balthasar.yaml
talosctl apply-config --mode auto -n 10.1.0.11 -e 10.1.0.11 -f generated/balthasar.yaml

# reach a node through another node's mesh when its uplink address is unreachable
talosctl -e 10.1.0.11 -n 10.1.3.13 get links

# clear etcd and Kubernetes state but keep the OS install and config (then bootstrap again)
talosctl reset --graceful=false --reboot --system-labels-to-wipe EPHEMERAL -n 10.1.0.11 -e 10.1.0.11 --wait=false
```

## UPS shutdown

The NUT server (`root@10.1.0.20`, UPS `forza`, Forza FX2200) is the primary. Each node runs the `siderolabs/nut-client`
extension (in the schematic) as a NUT secondary: `config/nut-client.yaml` -> `generated/nut-client.yaml` holds the `upsmon.conf`
with `SHUTDOWNCMD "/sbin/poweroff"` (machined's graceful shutdown). On a forced shutdown (FSD) all three nodes power off and
the primary shuts down after they disconnect.

FSD happens only when the UPS reports low battery; there is no on-battery timer, because a shutdown before the battery is
empty means the UPS never cuts and restores power, so the cluster would not start again.
The nodes log in as the server's existing `observer` user (`upsmon secondary`, password = `generated/nut-password`).

```sh
talosctl -n 10.1.0.11,10.1.0.12,10.1.0.13 -e 10.1.0.10 service ext-nut-client
talosctl -n 10.1.0.11 -e 10.1.0.11 logs ext-nut-client
ssh root@10.1.0.20 'upsc forza@localhost ups.status; upsmon -c fsd'   # the second command powers everything off
```

## Tuning and gotchas

Everything below was needed to make the build work; none of it is obvious from the defaults.

**Talos config**
- **Patching can't override some generated fields.** The generated `HostnameConfig` has `auto: stable`, and a
  hostname can't coexist with it; the generated `KubeNodeConfig` has the control-plane `NoSchedule` taint. A
  plain merge doesn't remove either, so the config files delete them with `$patch: delete` under the key
  (`auto:` in each node file, `taints.node-role.kubernetes.io/control-plane:` in `common.yaml`). Don't use
  `allowSchedulingOnControlPlanes`: it conflicts with the new `KubeNodeConfig` document.
- **IPv6 addresses ending in `::` must be quoted** in YAML (`address: "fd7a:2201:1ad::"`).
- **Disks by serial** (`UnattendedInstallConfig` `diskSelector: disk.serial == "..."`); device names swap
  between boots (see `hardware.md`).
- **etcd and kubelet use the `lo` addresses only**: `cluster.etcd.advertisedSubnets` and
  `KubeNodeConfig.nodeIP.validSubnets` are `10.1.3.8/29` and `fd7a:2201:1ad::10/124`. Without it the kubelet
  picks its node IP from the pod CIDRs and may choose the wrong NIC.
- **kube-proxy is off** (`KubeProxyConfig enabled: false`) because Gateway API needs kube-proxy replacement.
  Talos only skips deploying it at bootstrap; on an already-bootstrapped cluster delete the leftover
  `kubectl -n kube-system delete ds kube-proxy`. Flannel is removed with `KubeFlannelCNIConfig` `$patch: delete`.
- **The static IPv6 default route needs its own metric** (`metric: 100` on the `bond-up` route to
  `fd7a:2201:1ab::1`). The routers' RAs install a default route via `fe80::1` at metric 1024; a static one at the
  same metric fails with `netlink: file exists`, and the route controller then retries with a 30-70 s back-off,
  which also delays every other route change on that node (BGP detours and pod routes came 28-42 s late, a
  pod-traffic black hole of 49 s after a cable cut). Watch for `RouteSpecController` failures in `talosctl dmesg`.
- **The control-plane VIP is IPv4 only** (`Layer2VIPConfig`, gratuitous ARP) and sits on `bond-up`.
- **Changing etcd addresses or pod/service CIDRs needs a state wipe.** Talos never updates etcd peer URLs of an
  existing member (`MemberUpdate` is not called), so a node with new addresses can't rejoin. Apply the new
  configs to all nodes, run the EPHEMERAL reset on each, then bootstrap once.

**Longhorn storage**
- **Data disk:** each node's 1 TB NVMe is a `UserVolumeConfig` named `longhorn` (xfs, by serial, `minSize` is required)
  in the node files, mounted at `/var/mnt/longhorn`; the Longhorn `defaultDataPath` is that path.
- **Kubelet mount:** the kubelet only sees `/var/mnt/longhorn` through an `extraMounts` bind (`rshared`). The 1.14
  `KubeletConfig` document has no `extraMounts` field and conflicts with `machine.kubelet`, so `common.yaml` deletes
  the generated `KubeletConfig` and sets the legacy `machine.kubelet` instead, repeating the `image` and
  `defaultRuntimeSeccompProfileEnabled: true` the deleted document carried. Applies without a reboot.
- **Storage network:** none. Longhorn traffic is pod-to-pod, and remote PodCIDRs are routed over the 10G DAC mesh (BGP,
  metric 0) with the `bond-up` statics (metric 2000) as the 2.5G fallback, so it already prefers 10G.

**Uplink bond**
- `bond-up` over `enp5s0` + `enp4s0` (802.3ad, `lacpRate: fast`, `layer3+4`, `miimon 100`, `updelay`/`downdelay 200`)
  must match the switch bonds. Addresses and the default routes (`10.1.0.1`, `fd7a:2201:1ab::1`) live on the
  bond. A bond with one cabled member works.
- After applying, traffic takes 40 to 80 s to pass while LACP negotiates; the kernel logs "No 802.3ad response"
  at link-up, which is only the moment before negotiation completes. Always apply bond changes in `try` mode.
- Two unbonded uplink ports on the same VLAN made one node's address unreachable; bond them or leave one unplugged.

**Mesh BGP**
- **Sessions are IPv6-only** (neighbors are the `/127` peer addresses). With an IPv4 session per peer as well,
  an IPv6 prefix learned over it gets an IPv4 next hop; Linux rejects the resulting mixed ECMP route
  (`netlink: invalid argument`) and installs no IPv6 route at all. IPv4 prefixes over IPv6 sessions get IPv6
  next hops, which Linux accepts.
- **No BFD.** BFD over IPv6 stays `down` in Talos 1.14.4, so failover relies on `holdTime: 9s`. Fail-back after a
  cable returns can take a couple of minutes (GoBGP connect-retry backoff).
- `advertise: [lo]` originates the `/32` and `/128` as host routes. Distinct ASNs per node (65011-65013) let the
  third node transit when a cable is pulled.
- **Pod routing path:** Cilium (AS 65021) peers over `veth-workload` with the Talos `workload` instance (AS
  65031, VRF `vrf-workload`, `passive`, `installRoutes: false`); `fabric` imports `10.244.0.0/16` and
  `fd7a:2201:1ae::/56` from it. Cilium never installs received routes, which is why this Talos hop exists.

**Cilium**
- **`devices` and `directRoutingDevice` are required**: the node IP (`10.1.3.x`) sits on `lo`, and without them
  the agent exits with "unable to determine direct routing device". If the uplink interface changes (for
  example a new bond name), change both, run `cilium-cli upgrade --version 1.20.4 --set ...` and restart:
  `kubectl -n kube-system rollout restart ds/cilium ds/cilium-envoy deploy/cilium-operator`.
- `bpf.hostLegacyRouting=true` is the Cilium-on-Talos recommendation; `ipam.mode=kubernetes`,
  `k8sServiceHost=localhost` / `k8sServicePort=7445` (KubePrism), no cgroup mount and no `SYS_MODULE` are the
  Talos prerequisites.
- `bgpControlPlane` must be enabled before the BGP resources in `overlays/magi/infrastructure/cilium` are applied.
- Last-resort path over the uplink: each node's `bond-up` carries static routes (metric 2000) to the other two nodes'
  `lo` addresses and PodCIDRs via their `10.1.0.x` / `fd7a:2201:1ab::x` addresses. The BGP mesh routes (kernel metric
  0, IPv6 1024) win while any DAC path exists; when both DACs of a node are down the statics take over and pod, etcd
  and kubelet traffic crosses the 2.5G uplink. The PodCIDRs in the node files must equal
  `kubectl get nodes -o custom-columns=N:.metadata.name,C:.spec.podCIDRs` (kube-controller-manager hands them out in
  join order: casper `10.244.0.0/24`, balthasar `.1.0/24`, melchior `.3.0/24`); change the routes if a rebuild
  allocates them differently. Applied with `--mode try` first, then `--mode auto`.

**Operating notes**
- `talosctl etcd members` hangs while etcd isn't running (before bootstrap); don't use it to test for that.
  `talosctl bootstrap` refuses with "etcd data directory is not empty" if it already ran, which is safe.
- `talosctl reset` and `reboot` don't accept `--insecure`; power-cycle by hand in maintenance mode.
