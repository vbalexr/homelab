# Talos Linux for Magi Cluster

Talos v1.12 cluster configuration with three control-plane nodes.

## Prerequisites

- `talosctl` v1.12
- 3 nodes with 3 NVMe drives each (1 small for OS, 2 large for storage)
- DNS: `magi.vbalex.com` → control-plane VIP

## Quick Setup

### 1. Create Custom Image Schematic

Upload to Image Factory:
```bash
curl -X POST --data-binary @cluster/magi/talos/image-factory-schematic.yaml \
  https://factory.talos.dev/schematics
# Returns: {"id":"YOUR_SCHEMATIC_ID"}
```

Update `config/common.yaml` with your schematic ID.

### 2. Generate Secrets

```bash
talosctl gen secrets -o cluster/magi/talos/talos-secrets.yaml
```

### 3. Verify Node Configs

Edit node files and confirm:
- Install disk = smallest NVMe (usually `/dev/nvme0n1`)
- `bond1` network = cluster-only (`10.255.0.0/24` + IPv6)

### 4. Generate Machine Configs

```bash
mkdir -p cluster/magi/talos/generated

talosctl gen config magi https://magi.vbalex.com:6443 \
  --with-secrets cluster/magi/talos/talos-secrets.yaml \
  --config-patch @cluster/magi/talos/config/common.yaml \
  --config-patch @cluster/magi/talos/config/node-balthasar.yaml \
  --output cluster/magi/talos/generated/balthasar.yaml \
  --output-types controlplane
```

Repeat for `casper.yaml` and `melchior.yaml`.

### 5. Bootstrap & Apply

```bash
# Boot nodes with ISO and apply configs
talosctl apply-config --insecure --nodes=<node-ip> \
  --file=cluster/magi/talos/generated/balthasar.yaml

# Wait for nodes to boot, then create kubeconfig
talosctl kubeconfig -n <node-ip>
```

## Key Notes

- All nodes are control-plane (HA etcd)
- Jumbo frames (MTU 9000) on `bond1`
- Cilium uses `bond0` (external) + `bond1` (pod networking)
- IPv4 must come before IPv6 in subnet configs

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
