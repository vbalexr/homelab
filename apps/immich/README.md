# Immich Application

Photo management and backup service with machine learning-powered features.

## Components

- **immich-server**: Main application server exposing web UI and API on port 2283
- **immich-machine-learning**: ML inference service for object detection, facial recognition, and other ML features

## Architecture

- Uses external PostgreSQL database (deployed in `postgres` namespace)
- Uses external Valkey/Redis cache (deployed in `postgres` namespace)
- Deployments (not StatefulSet) for horizontal scalability
- Shared storage (ceph-filesystem RWX) for:
  - Upload media: `/data`
  - ML model cache: `/cache`

## Configuration

Environment variables are injected from secrets and configmaps:
- Database connection details sourced from `immich-secret`
- ML model cache location and redis connection configurable

## Database Setup

The Immich application requires a dedicated PostgreSQL user and database with specific extensions enabled. Connect to the PostgreSQL instance and run:

```sql
CREATE USER home_immich WITH PASSWORD 'password';

CREATE DATABASE home_immich;
\c home_immich
BEGIN;
ALTER DATABASE home_immich OWNER TO home_immich;
CREATE EXTENSION vchord CASCADE;
CREATE EXTENSION earthdistance CASCADE;
COMMIT;
```

**Note**: 
- Replace `home_immich` user and database name if using a different namespace or configuration
- Replace `'password'` with a strong, random password and store it in the `immich-secret` as `DB_PASSWORD`
- The `vchord` and `earthdistance` extensions are required for Immich's vector search and geo-location features
- These commands assume PostgreSQL is deployed with the vchord extension available (see `apps/postgres`)

## References

- [Immich Documentation](https://docs.immich.app/)
- [Docker Compose Reference](https://github.com/immich-app/immich/blob/main/docker/docker-compose.yml)

