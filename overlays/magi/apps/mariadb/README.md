# mariadb

Single instance, StatefulSet `mariadb` in namespace `database`: MariaDB 11.4 (`docker.io/library/mariadb:11.4`), 20Gi on
`longhorn-retain`, LoadBalancer `10.1.1.21` / `fd7a:2201:1ac::21` port 3306 (`externalTrafficPolicy: Local`). Databases:
`auth`, `buyback`, `universe` (account `mrakaki`) and `sspti_aa` (account `sspti_aa`), both accounts `@'%'`.

- **Config:** `innodb.cnf` from the ConfigMap is mounted on `/etc/mysql/conf.d` and applied (buffer pool 7G, 512M redo log,
  `O_DIRECT`). Check with `SHOW VARIABLES`; charset `utf8mb4`, collation `utf8mb4_uca1400_ai_ci` and the default `sql_mode` are the image defaults.
- **Root password:** `MARIADB_ROOT_PASSWORD` from the `mariadb-credentials` Secret is only used when the data directory is empty.
- **Restore from a dump:** `mariadb-dump --single-transaction --routines --events --triggers --databases auth buyback sspti_aa universe`
  plus `SHOW CREATE USER` / `SHOW GRANTS` of the two accounts (keeps the password hashes). About 4 GB of SQL takes about 45 minutes;
  run the client with `< file` (no `podman exec -i` pipe) and set `innodb_flush_log_at_trx_commit=2` meanwhile, back to `1` afterwards.
- **Check a restore:** compare `count(*)` of every table between the old and new server.
- **Clients on VLAN 10 (10.1.0.0/24):** the nodes are on the same L2 as such hosts, so the reply from a node to a client on VLAN 10
  skips the router that carried the request. A stateful router then drops the client's packets once about 64 KB were sent on a
  connection, and the session hangs. Clients outside VLAN 10, or a host route to the node that runs the pod, avoid it.
