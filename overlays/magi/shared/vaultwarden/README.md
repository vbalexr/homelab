# vaultwarden

Deployment `vaultwarden` in namespace `shared`: image `ghcr.io/dani-garcia/vaultwarden:1.37.2`, listens on 8080 (non-root),
`/data` on PVC `vaultwarden-data` (2Gi, `longhorn-retain`: `config.json` with the admin settings, `rsa_key.pem`, `icon_cache`).
Database `shared_vaultwarden` (role `shared_vaultwarden`) on `postgres.database.svc.cluster.local:5432`, URL in the SOPS secret
`vaultwarden-env`. Public name `vault.vbalex.com` through the namespace's `shared-gateway` (10.1.1.10, certificate `shared-gateway-tls`, http redirects
to https).

## Cutover from podman (10.1.0.25)

    D=/srv/podman/migration/vaultwarden/$(date +%Y%m%d-%H%M%S)
    ssh root@10.1.0.25 "umask 077; mkdir -p $D; podman stop vaultwarden; podman update --restart=no vaultwarden
      podman exec postgres pg_dump -U postgres -Fc shared_vaultwarden > $D/db.dump
      podman exec postgres pg_dumpall -U postgres --roles-only | grep -E '^(CREATE|ALTER) ROLE shared_vaultwarden( |;)' > $D/role.sql
      tar -C /srv/podman/stacks/vaultwarden -cf $D/data.tar data"
    kubectl -n shared scale deploy vaultwarden --replicas=0
    P="kubectl -n database exec -i postgres-0 -- psql -v ON_ERROR_STOP=1 -U postgres -d postgres"
    $P -c "select pg_terminate_backend(pid) from pg_stat_activity where datname='shared_vaultwarden'"
    $P -c "drop database if exists shared_vaultwarden" -c "drop role if exists shared_vaultwarden"
    ssh root@10.1.0.25 "cat $D/role.sql" | $P -q
    $P -c "create database shared_vaultwarden owner shared_vaultwarden"
    ssh root@10.1.0.25 "cat $D/db.dump" | kubectl -n database exec -i postgres-0 -- pg_restore -U postgres -d shared_vaultwarden --no-owner --role=shared_vaultwarden
    kubectl -n shared scale deploy vaultwarden --replicas=1 && kubectl -n shared rollout status deploy vaultwarden
    ssh root@10.1.0.25 "cat $D/data.tar" | kubectl -n shared exec -i deploy/vaultwarden -- tar -x --strip-components=1 -C /data --no-same-owner
    kubectl -n shared rollout restart deploy vaultwarden

(`pg_restore` reports 3 ignored `ALTER DEFAULT PRIVILEGES FOR ROLE postgres` errors: harmless.)
Verify: row counts per table equal the source, `curl --resolve vault.vbalex.com:443:10.1.1.10 https://vault.vbalex.com/alive`
returns 200, no errors in `kubectl -n shared logs deploy/vaultwarden`. Then point Caddy (`reverse_proxy 10.1.1.10:443` with
`tls_server_name vault.vbalex.com`) or DNS at 10.1.1.10.

Rollback: `kubectl -n shared scale deploy vaultwarden --replicas=0; ssh root@10.1.0.25 podman start vaultwarden`.
