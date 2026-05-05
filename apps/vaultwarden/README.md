# Vaultwarden

Self-hosted Bitwarden-compatible server. Uses PostgreSQL, stores data under `/data`.

```sql
CREATE USER home_vaultwarden WITH PASSWORD 'password';
CREATE DATABASE home_vaultwarden OWNER home_vaultwarden;
```

Set `DATABASE_URL` in overlay secret and `DOMAIN` in ConfigMap.
