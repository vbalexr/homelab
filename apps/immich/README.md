# Immich Application

Photo management with ML features. Server on port 2283, ML inference service.

Requires PostgreSQL with vchord and earthdistance extensions. Uses Valkey/Redis cache.

```sql
CREATE USER home_immich WITH PASSWORD 'password';
CREATE DATABASE home_immich OWNER home_immich;
CREATE EXTENSION vchord CASCADE;
CREATE EXTENSION earthdistance CASCADE;
```

