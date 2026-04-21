# PostgreSQL 16 with VectorChord

Base Kubernetes manifests for PostgreSQL 16 + pgvector + VectorChord.

## Key Features

- **Storage**: 50Gi Rook Ceph RBD
- **StatefulSet**: 1 replica with persistent data
- **Probes**: Liveness & Readiness configured
- **Extensions**: pgvector, VectorChord auto-loaded
- **Security**: Non-root container (uid: 999)

## Customization

Use overlays to customize per-cluster:

```bash
kubectl apply -k overlays/magi/databases/postgres/
```

Common patches in overlays:
- Image tag override
- Resource requests/limits
- Replica count
- Storage size
- Resource limits (patch StatefulSet)
- Database credentials (patch Secret)
- Storage class or size (patch PVC)
- PostgreSQL configuration (patch ConfigMap)
