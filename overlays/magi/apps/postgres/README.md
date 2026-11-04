# postgres

Single instance, StatefulSet `postgres` in namespace `database`: Postgres 18 with `vector` and `vchord`, 10Gi on
`longhorn-retain`, LoadBalancer `10.1.1.20` / `fd7a:2201:1ac::20` port 5432 (`externalTrafficPolicy: Local`, so pg_hba sees the
real client address). Databases: `shared_immich`, `shared_vaultwarden`, `verdesumergido`, `netbox`.

- **Config:** `/etc/postgresql/conf.d` is read through an `include_dir` that an init script appends to `postgresql.conf`.
  `shared_preload_libraries=vector,vchord` is also a container argument, because the init scripts run against a temporary
  server that ignores the include.
- **Restore from a `pg_dumpall`:** the netbox ltree triggers need `search_path=public` while restoring; rewrite the
  `set_config('search_path'...)` lines in the netbox section of the dump, otherwise 11 triggers fail. Restoring takes about
  2 minutes per GB.
- **Check a restore:** compare `count(*)` of every table between the old and new server.
