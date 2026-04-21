# Valkey

Valkey is a high-performance data structure store, used as a database, cache, and message broker. It's a Redis fork maintained by the Linux Foundation.

## Configuration

- Default port: 6379
- Persistence: AOF (Append Only File) enabled with RDB snapshots
- Password authentication required

## Usage

The StatefulSet creates a single Valkey instance with persistent storage. The service is headless to support StatefulSet DNS discovery.

Connection string: `valkey-0.valkey.<namespace>.svc.cluster.local:6379`
