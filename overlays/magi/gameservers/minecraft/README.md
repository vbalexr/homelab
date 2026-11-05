# minecraft

Paper 26.3 (build 194) with Geyser, Floodgate and Simple Voice Chat, image `itzg/minecraft-server:java25` pinned by digest.

- StatefulSet, one replica, data on PVC `minecraft-data` (`longhorn-retain`, RWO): the volume can only be attached to one
  pod, so two instances never run against the same world.
- JVM heap `MEMORY=4G`; container request = limit = 6Gi (heap plus metaspace, threads and native memory).
- Service `minecraft` (LoadBalancer 10.1.1.15, fd7a:2201:1ac::15): TCP 25565 (Java), UDP 19132 (Bedrock/Geyser), UDP 24454 (voice chat).
- `externalTrafficPolicy: Local` so the server sees the real player address (logs, `banned-ips`, Geyser and the voice chat);
  with one replica only the node running the pod advertises the address over BGP.
- RCON is enabled in `server.properties` but not exposed by the Service; the password stays in the volume (`rcon-cli` inside the pod).
  Whitelist is off (`white-list=false`); ops are in `ops.json` on the volume. No secrets in the manifests.
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
