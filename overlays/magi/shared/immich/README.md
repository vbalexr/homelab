# immich

Deployments `immich-server` (`ghcr.io/immich-app/immich-server:v3.1.0`, port 2283) and `immich-machine-learning` (same tag, 3003) in
namespace `shared`, both running as 1000:100. Public name `photos.vbalex.com` through `shared-gateway` (10.1.1.10); the route has no
request timeout so large uploads are not cut.

- **Library:** the NAS export `10.1.0.15:/mnt/Vault/immich` (about 800 GB) is mounted as PV/PVC `immich-library` (NFS 4.1, Retain) on `/data`. It is not
  copied into the cluster. The export must allow the node uplink addresses 10.1.0.11-13.
- **Models:** PVC `immich-ml-cache` (5Gi, `longhorn-retain`); an empty cache is refilled by the first smart-search job.
- **Database:** `shared_immich` (role `shared_immich`, extensions `vector` and `vchord`) on `postgres.database.svc.cluster.local`.
- **Queue:** Valkey `valkey.database.svc.cluster.local` db 0. Passwords are in the SOPS secret `immich-env` (`DB_PASSWORD`, `REDIS_PASSWORD`).
- Never run the cluster copy and a podman Immich against the same library or database.

## Restore a database dump

    kubectl -n shared scale deploy immich-server --replicas=0
    P="kubectl -n database exec -i postgres-0 -- psql -v ON_ERROR_STOP=1 -U postgres -d postgres"
    $P -c "select pg_terminate_backend(pid) from pg_stat_activity where datname='shared_immich'" -c "drop database if exists shared_immich"
    $P -c "create database shared_immich owner shared_immich"
    cat shared_immich.dump | kubectl -n database exec -i postgres-0 -- pg_restore -U postgres -d shared_immich --exit-on-error

(`pg_dump -Fc` of the old database; the role `shared_immich` must exist with the password from the secret.) Check `count(*)` of `asset` and `album`.
