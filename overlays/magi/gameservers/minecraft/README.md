# minecraft

Paper 26.3 (build 194) with Geyser, Floodgate and Simple Voice Chat, image `itzg/minecraft-server:java25` pinned by digest.

- StatefulSet, one replica, data on PVC `minecraft-data` (`longhorn-retain`, RWO): the volume can only be attached to one
  pod, so two instances never run against the same world.
- Survival, `DIFFICULTY=easy` with `SPAWN_MONSTERS=false` (no hostile mobs, vanilla hunger and regeneration), `MAX_PLAYERS=50`; ops come from `OPS` (merged into `ops.json`).
- JVM heap `MEMORY=4G`; container request = limit = 6Gi (heap plus metaspace, threads and native memory), CPU request 1500m, no CPU limit.
- Sidecar `guide` (`nginx-unprivileged`, port 8080) serves the player guide from ConfigMap `minecraft-guide`, generated from
  [guide/index.html](guide/index.html) without a name hash, so edits reach the pod without restarting the server (about a minute).
  The page serves `CoyoteDensPack-1.2.1.mrpack`: the official Modrinth pack "Satisfaction Guaranteed" 1.2.1 (Fabric, 26.3, client-side only,
  so it works against the Paper server) with exactly four changes: the name, Simple Voice Chat added, the server in the multiplayer
  list, and `icon.png` at the pack root (from `icon/pack-icon.png`; the format has no icon field, so launchers may ignore it). Mods stay pinned to the official versions (the pack lists Modrinth URLs with hashes; the launcher downloads them), so only the
  small file is served from the ConfigMap (1 MiB limit). Don't add or drop mods: compatibility between them is the hard part.
  It is built locally, nothing runs in the cluster: `python3 build-mrpack.py` in `guide/` (it downloads and hash-checks the official
  file). To move to a new official release, change `PACK_VERSION` and the file name in `kustomization.yaml` and the page.
  The same logo is the guide page's favicon and header image (`guide/logo.svg`), and the 64x64 PNG `icon/server-icon.png.bin` (rendered from
  `icon/server-icon.svg` with `rsvg-convert -w 64 -h 64`; kept as `.bin` because Flux skips `.png` and `.zip` files by default) is the multiplayer-list icon: ConfigMap `minecraft-icon` mounted over
  `/data/server-icon.png` (name hash on, so a new icon restarts the pod, which only reads it at start).
- Service `minecraft` (LoadBalancer, `minecraft.vbalex.com`): TCP 80 (redirects to HTTPS), TCP 443 (guide, certificate `minecraft-tls` from cert-manager, nginx reloads it on renewal), TCP 25565 (Java),
  UDP 19132 (Bedrock/Geyser), UDP 24454 (voice chat).
- `externalTrafficPolicy: Local` so the server sees the real player address (logs, `banned-ips`, Geyser and the voice chat);
  with one replica only the node running the pod advertises the address over BGP.
- RCON is not exposed by the Service; use `rcon-cli` inside the pod. No secrets in the manifests.
- Plugins are downloaded by the image from the `PLUGINS` URLs (already present in the volume), so the pod needs internet egress.

## Console and backups

    kubectl -n gameservers exec -it minecraft-0 -- rcon-cli
    # consistent copy while running
    kubectl -n gameservers exec minecraft-0 -- sh -c 'rcon-cli save-off && rcon-cli save-all flush'
    kubectl -n gameservers exec minecraft-0 -- tar czf - -C /data . > minecraft-data.tar.gz
    kubectl -n gameservers exec minecraft-0 -- rcon-cli save-on

## Restore into an empty volume

Scale the StatefulSet to 0 (suspend the Flux kustomization `gameservers` first), mount `minecraft-data` in a pod running as uid/gid 1000
(fsGroup 1000) and `tar xzf - -C /data`.
