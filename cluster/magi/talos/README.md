# Talos install for Magi

This guide uses Talos v1.12 and the config patches in this directory.

## Prereqs
- `talosctl` v1.12 on your workstation
- DNS for `magi.vbalex.com` points to the control-plane VIP or a control-plane node
- Talos ISO or PXE boot for each node
- Each node has 3 NVMe drives: 1 small (<512GB) for OS, 2 larger (>=512GB) for Longhorn storage
- This configuration includes Longhorn requirements (iscsi-tools, util-linux-tools, V2 data engine support)

## 1) Create custom Talos image with Image Factory
Talos v1.12+ uses Image Factory to create custom boot assets with extensions and kernel arguments baked in.

Upload the schematic to get a custom image URL:

```sh
curl -X POST --data-binary @cluster/magi/talos/image-factory-schematic.yaml \
  https://factory.talos.dev/schematics

# This returns a JSON response with the schematic ID, for example:
# {"id":"YOUR_SCHEMATIC_ID"}
```

Update [cluster/magi/talos/config/common.yaml](cluster/magi/talos/config/common.yaml) with your schematic ID:
- Replace `YOUR_SCHEMATIC_ID` in `machine.install.image` with the ID returned above
- The image URL format is: `factory.talos.dev/installer/YOUR_SCHEMATIC_ID:v1.12.4`

Alternatively, use the web interface at https://factory.talos.dev to generate the schematic visually.

The schematic includes:
- **Extensions**: i915-ucode, intel-ucode, nut-client, gasket-driver

## 2) Prepare secrets (gitignored)
Generate the Talos secrets file once and keep it out of git:

```sh
talosctl gen secrets -o cluster/magi/talos/talos-secrets.yaml
```

## 3) Update per-node patches
Edit the node files and confirm the install disk matches your OS disk (should be the smallest NVMe, typically `/dev/nvme0n1`):
- [cluster/magi/talos/config/node-balthasar.yaml](cluster/magi/talos/config/node-balthasar.yaml)
- [cluster/magi/talos/config/node-casper.yaml](cluster/magi/talos/config/node-casper.yaml)
- [cluster/magi/talos/config/node-melchior.yaml](cluster/magi/talos/config/node-melchior.yaml)

Each node config must include `kubelet.nodeIP.validSubnets` pointing to the bond1 network (10.255.0.0/24,fd7a:2201:7351::/64) for pod-to-pod communication. Cilium will use bond0 for external LoadBalancer traffic (BGP) and bond1 for pod networking via native routing.

Important (address family + ordering):
- Keep `machine.kubelet.nodeIP.validSubnets` ordered with IPv4 first (`10.255.0.0/24` before `fd7a:2201:7351::/64`).
- Keep `bond1.addresses` ordered with IPv4 first (`10.255.0.x/24` before `fd7a:2201:7351::x/64`).

If IPv6 is selected as the node "public address" while your `serviceSubnets` are IPv4-first, `kube-apiserver` can exit with an error like:

```text
service IP family "10.42.0.0/16" must match public address family "fd7a:2201:7351::1"
```

Network note:
- `bond1` is cluster-only and untagged in Talos (no VLAN subinterfaces on `bond1`).
- `bond1` is configured for jumbo frames (`mtu: 9000`). Ensure the full L2 path supports jumbo frames (switch ports/MLAG + NICs), otherwise you can see drops/retransmits.

Optional adjustments (if needed):
- Bond mode (`active-backup` vs `802.3ad`)
- Default route and DNS servers under `machine.network`
- Install disk device serial
- Aditional disk devices serials

## 4) Generate machine configs
Generate one config per node:

```sh
mkdir -p cluster/magi/talos/generated

# If you need to re-run this and overwrite existing files, add `--force`.

talosctl gen config magi https://magi.vbalex.com:6443 \
  --with-secrets cluster/magi/talos/talos-secrets.yaml \
  --config-patch @cluster/magi/talos/config/common.yaml \
  --config-patch @cluster/magi/talos/config/node-balthasar.yaml \
  --output cluster/magi/talos/generated/balthasar.yaml \
  --output-types controlplane

talosctl gen config magi https://magi.vbalex.com:6443 \
  --with-secrets cluster/magi/talos/talos-secrets.yaml \
  --config-patch @cluster/magi/talos/config/common.yaml \
  --config-patch @cluster/magi/talos/config/node-casper.yaml \
  --output cluster/magi/talos/generated/casper.yaml \
  --output-types controlplane

talosctl gen config magi https://magi.vbalex.com:6443 \
  --with-secrets cluster/magi/talos/talos-secrets.yaml \
  --config-patch @cluster/magi/talos/config/common.yaml \
  --config-patch @cluster/magi/talos/config/node-melchior.yaml \
  --output cluster/magi/talos/generated/melchior.yaml \
  --output-types controlplane
```

Generate the talosconfig once:

```sh
talosctl gen config magi https://magi.vbalex.com:6443 \
  --with-secrets cluster/magi/talos/talos-secrets.yaml \
  --output cluster/magi/talos/talosconfig \
  --output-types talosconfig
```

## 5) Boot nodes and apply configs
Boot each node using the custom Talos image from Image Factory. You can download the ISO from:
```
https://factory.talos.dev/image/YOUR_SCHEMATIC_ID/v1.12.4/metal-amd64.iso
```

Or use PXE boot with the kernel and initramfs from the factory.

Then apply the matching config to each node using its current DHCP IP (replace `<dhcp-ip>` with discovered IPs):

```sh
# Baltasar
talosctl apply-config \
  -n IP \
  -f cluster/magi/talos/generated/balthasar.yaml \
  --insecure

# Casper
talosctl apply-config \
  -n IP \
  -f cluster/magi/talos/generated/casper.yaml \
  --insecure

# Melchior
talosctl apply-config \
  -n IP \
  -f cluster/magi/talos/generated/melchior.yaml \
  --insecure
```

After applying configs, the nodes will install Talos and reboot. They will come up with their static IPs (10.1.0.6, 10.1.0.7, 10.1.0.8).

## 6) Bootstrap the cluster
Bootstrap once, from any control-plane node:

```sh
TALOSCONFIG=cluster/magi/talos/talosconfig talosctl bootstrap -n 10.1.0.6 -e 10.1.0.6
```

## 7) Get kubeconfig

```sh
TALOSCONFIG=cluster/magi/talos/talosconfig talosctl kubeconfig -n 10.1.0.6 -e 10.1.0.6
```

## 8) Install Cilium

```sh
cilium install \
    --set ipv6.enabled=true \
    --set ipam.mode=kubernetes \
    --set kubeProxyReplacement=true \
    --set securityContext.capabilities.ciliumAgent="{CHOWN,KILL,NET_ADMIN,NET_RAW,IPC_LOCK,SYS_ADMIN,SYS_RESOURCE,DAC_OVERRIDE,FOWNER,SETGID,SETUID}" \
    --set securityContext.capabilities.cleanCiliumState="{NET_ADMIN,SYS_ADMIN,SYS_RESOURCE}" \
    --set cgroup.autoMount.enabled=false \
    --set cgroup.hostRoot=/sys/fs/cgroup \
    --set k8sServiceHost=localhost \
    --set k8sServicePort=7445 \
    --set gatewayAPI.enabled=true \
    --set gatewayAPI.enableAlpn=true \
    --set gatewayAPI.enableAppProtocol=true \
    --set cni.exclusive=false \
    --set bgpControlPlane.enabled=true \
    --set autoDirectNodeRoutes=true \
    --set cluster.name=magi \
    --set operator.replicas=1 \
    --set routingMode=native \
    --set ipv4NativeRoutingCIDR=10.41.0.0/16 \
    --set ipv6NativeRoutingCIDR=fd7a:41::/48 \
    --set devices="bond0" \
    --set directRoutingDevice=bond1 \
    --set tunnelProtocol=vxlan
```

**Important**: 
- `devices=bond0` ensures Cilium attaches to the external-facing bond where LoadBalancer/BGP traffic arrives
- `directRoutingDevice=bond1` ensures node routes / native routing use the 10Gb bond
- Native routing uses bond1 (10.255.0.0/24,fd7a:2201:7351::/64) for pod-to-pod traffic and bond0 for external traffic
