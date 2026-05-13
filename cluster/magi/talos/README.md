# Talos Linux for Magi Cluster

Talos v1.12 with three control-plane nodes.

## Setup

1. Create the Image Factory schematic and update Talos installer image
    - From `cluster/magi/talos`, upload the schematic:
    - `curl -X POST --data-binary @image-factory-schematic.yaml https://factory.talos.dev/schematics`
    - The response contains a schematic ID (for example `9d8a...0cf8`).
    - Update `config/common.yaml` under `machine.install.image` to use that ID:
      - `factory.talos.dev/installer/<SCHEMATIC_ID>:v1.13.0`
    - Keep the schematic file and `common.yaml` in sync whenever kernel modules/extensions change.

2. Generate cluster secrets (one-time per cluster)
    - Run from the repository root:
    - `talosctl gen secrets -o cluster/magi/talos/talos-secrets.yaml`
    - Commit/store this file securely. Re-generating it later creates a different trust root.

3. Generate Talos control-plane configs with node-based filenames
    - Run from `cluster/magi/talos`.
    - Generate one file per node (control-plane output only):
    - `talosctl gen config magi https://magi.vbalex.com:6443 --with-secrets talos-secrets.yaml --config-patch @config/common.yaml --config-patch-control-plane @config/node-balthasar.yaml -t controlplane -o generated/balthasar.yaml`
    - `talosctl gen config magi https://magi.vbalex.com:6443 --with-secrets talos-secrets.yaml --config-patch @config/common.yaml --config-patch-control-plane @config/node-casper.yaml -t controlplane -o generated/casper.yaml`
    - `talosctl gen config magi https://magi.vbalex.com:6443 --with-secrets talos-secrets.yaml --config-patch @config/common.yaml --config-patch-control-plane @config/node-melchior.yaml -t controlplane -o generated/melchior.yaml`
    - Generate `talosconfig` once (used in bootstrap and kubeconfig steps):
    - `talosctl gen config magi https://magi.vbalex.com:6443 --with-secrets talos-secrets.yaml -t talosconfig -o talosconfig`

4. Apply generated config to each node (initial bootstrap uses `--insecure`)
    - Apply each node file to its matching IP:
    - `talosctl apply-config --insecure --nodes 10.1.0.6 --file generated/balthasar.yaml`
    - `talosctl apply-config --insecure --nodes 10.1.0.7 --file generated/casper.yaml`
    - `talosctl apply-config --insecure --nodes 10.1.0.8 --file generated/melchior.yaml`
    - After apply, nodes install Talos and reboot automatically.

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
    --set bpf.masquerade=true \
    --set enableIPv6Masquerade=true \
    --set loadBalancer.mode=snat \
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
