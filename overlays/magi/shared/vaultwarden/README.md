# vaultwarden

Deployment `vaultwarden` in namespace `shared`, listening on 8080 (non-root), `/data` on PVC `vaultwarden-data` (`longhorn-retain`:
`config.json` with the admin settings, `rsa_key.pem`, `icon_cache`). Database `shared_vaultwarden` (role `shared_vaultwarden`) on the
cluster postgres, URL in the SOPS secret `vaultwarden-env`. Published through the namespace's `shared-gateway`.

## Restore

Scale the deployment to 0, recreate the role and database on postgres, `pg_restore` a `pg_dump -Fc` of `shared_vaultwarden` into it
(`--no-owner --role=shared_vaultwarden`; three ignored `ALTER DEFAULT PRIVILEGES FOR ROLE postgres` errors are harmless), scale back to 1,
then extract the saved `data` directory into `/data` and restart. Verify row counts and `/alive`.
