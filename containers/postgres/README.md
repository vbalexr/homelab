# PostgreSQL 16 with VectorChord Container

Production-ready PostgreSQL 16 Docker image with pgvector and VectorChord vector database extensions pre-installed, optimized for homelab environments.

## Components

### 1. Container Image
- **Base**: PostgreSQL 16.3 on Debian Bookworm
- **Extensions**: 
  - **pgvector** (v0.5.1) - Vector similarity search
  - **VectorChord** (v0.2.1) - Advanced vector indexing
- **Multi-stage build**: Compiled extensions for minimal final image size
- **Health checks**: Built-in PostgreSQL readiness probes

### 2. GitHub Actions Workflows

#### `build-postgres-image.yml` - Container Build Pipeline
**Triggers**: 
- Push to `main` branch (any change in `containers/postgres/**`)
- Git tags matching `v*`
- Manual dispatch via GitHub UI

**Capabilities**:
- Multi-architecture builds (linux/amd64, linux/arm64)
- Automatic semantic versioning
- Builds cached for faster iterations
- Pushes to `ghcr.io/vbalexr/postgres:latest` + version tags

#### `check-postgres-updates.yml` - Automated Dependency Updates
**Schedule**: Every Monday at 02:00 UTC

**Process**:
1. Queries Docker Hub API for latest PostgreSQL 16.x release
2. Queries GitHub API for latest pgvector release
3. Queries GitHub API for latest VectorChord release
4. Compares with versions in `containers/postgres/version.txt`
5. If updates found:
   - Updates `version.txt`
   - Auto-commits with detailed change log
   - Triggers `build-postgres-image.yml` workflow
   - Generates GitHub Actions summary

**Outputs**: Automatic version bumps, weekly build cycles if updates available

### 3. Kubernetes Manifests

#### Base Manifests (`apps/postgres/`)
- **Namespace**: `postgres` (isolated from other workloads)
- **StatefulSet**: Single replica PostgreSQL pod with:
  - Volume mounts for data, config, and init scripts
  - Liveness/readiness probes (pg_isready)
  - Security context (non-root, FSGroup 999)
  - Resource requests: 500m CPU, 512Mi memory
  - Resource limits: 2 CPU, 2Gi memory
- **Service**: Headless service for DNS discovery
- **PersistentVolumeClaim**: 20Gi using Rook Ceph RBD storage
- **ConfigMaps**: 
  - PostgreSQL configuration (postgresql.conf, pg_hba.conf)
  - Initialization SQL scripts (extensions, sample table setup)
- **Secret**: Database credentials (plaintext in base, encrypted in overlay)

#### Overlay Configuration (`overlays/magi/databases/postgres/`)
- **Patches**:
  - StatefulSet resource limits customized for magi cluster (1CPU, 1Gi memory requests)
  - Secret password override with cluster-specific credentials
- **SOPS Integration**: Secrets encrypted with age key from `.sops.yaml`
- **Labels**: Cluster identification tags (`cluster: magi`, `environment: homelab`)

### 4. FluxCD Integration
**Manifest**: `cluster/magi/databases.yaml`

New Kustomization resource that:
- Points to `overlays/magi/databases`
- Enables SOPS decryption via `sops-age` secret
- Depends on `infrastructure` kustomization (ensures Rook Ceph ready before PostgreSQL deployment)
- Reconciles every 10 minutes

## Deployment Quick Start

### Prerequisites
- Kubernetes cluster with Rook Ceph storage (already available in your magi cluster)
- FluxCD bootstrapped with SOPS age key (`sops-age` secret in flux-system namespace)
- GitHub Container Registry (GHCR) credentials for image pulls

### Manual Deployment (Local Testing)

```bash
# Validate base manifests
kustomize build apps/postgres/

# Validate overlay manifests
kustomize build overlays/magi/databases/postgres/

# Deploy base only (without secrets encryption)
kubectl apply -k apps/postgres/

# Deploy overlay (with SOPS-encrypted secrets)
kubectl apply -k overlays/magi/databases/postgres/
```

### Automatic Deployment via FluxCD

1. **Initial Setup**: Push changes to `main` branch
2. **FluxCD Reconciliation**: 
   ```bash
   flux reconcile kustomization databases --with-source
   ```
3. **Verify Deployment**:
   ```bash
   kubectl get statefulset -n postgres
   kubectl get pods -n postgres
   kubectl logs -f -n postgres postgres-0
   ```

## Configuration Customization

### Change PostgreSQL Version
1. Edit `containers/postgres/version.txt`:
   ```
   PG_VERSION=16.4
   PGVECTOR_VERSION=0.5.2
   VECTORCHORD_VERSION=0.3.0
   ```
2. Push to main → GitHub Actions rebuilds automatically

### Change Database Password
1. Edit `overlays/magi/databases/postgres/secret-patch.yaml`:
   ```yaml
   stringData:
     password: "YourStrongPassword123!"
   ```
2. Use SOPS to encrypt before committing:
   ```bash
   sops overlays/magi/databases/postgres/secret-patch.yaml
   ```
3. Commit and push → FluxCD applies encrypted secret

### Adjust Resource Limits
Edit `overlays/magi/databases/postgres/statefulset-patch.yaml`:
```yaml
resources:
  requests:
    cpu: 2
    memory: 2Gi
  limits:
    cpu: 4
    memory: 4Gi
```

### Modify PostgreSQL Configuration
Edit `apps/postgres/configmap.yaml` - `postgres-config` ConfigMap:
- `postgresql.conf`: Query optimization, memory settings, WAL configuration
- `pg_hba.conf`: Authentication rules and allowed client connections

## Version Tracking

**File**: `containers/postgres/version.txt`

Used by both workflows to determine current versions and trigger updates. Format:
```
PG_VERSION=16.3
PGVECTOR_VERSION=0.5.1
VECTORCHORD_VERSION=0.2.1
BUILD_DATE=2026-04-17
```

## Security Notes

1. **Non-root container**: PostgreSQL runs as UID 999 (postgres user)
2. **Read-only root filesystem**: Disabled (PostgreSQL needs write access to /var/run)
3. **Drop all capabilities**: Prevents privilege escalation
4. **Secret encryption**: SOPS age encryption applied via FluxCD
5. **Network policies**: Consider adding NetworkPolicies to restrict pod-to-pod traffic
6. **Default password**: Change from `changeme` before production deployment

## Troubleshooting

### Pod fails to start
```bash
kubectl describe pod -n postgres postgres-0
kubectl logs -n postgres postgres-0
```

### Storage not provisioning
```bash
kubectl get pvc -n postgres
kubectl describe pvc -n postgres postgres-data-postgres-0
```

### Extensions not loading
```bash
kubectl exec -it -n postgres postgres-0 -- psql -U postgres -c \
  "SELECT extname FROM pg_extension WHERE extname IN ('pgvector', 'vectorchord');"
```

### Verify version running
```bash
kubectl exec -n postgres postgres-0 -- psql -U postgres -c "SELECT version();"
```

## Files Overview

```
homelab/
├── containers/postgres/           # Container build context
│   ├── Dockerfile                 # Multi-stage PostgreSQL 16 + extensions build
│   ├── .dockerignore              # Docker build exclusions
│   ├── version.txt                # Current dependency versions
│   ├── init-db.sql                # Database initialization script
│   └── README.md                  # This file
├── apps/postgres/                 # Base Kubernetes manifests
│   ├── kustomization.yaml         # Base kustomization
│   ├── namespace.yaml             # postgres namespace
│   ├── statefulset.yaml           # StatefulSet definition
│   ├── service.yaml               # Headless service
│   ├── pvc.yaml                   # Storage definition
│   ├── configmap.yaml             # Configuration files
│   └── secret.yaml                # Secret template (override in overlay)
├── overlays/magi/databases/       # Cluster-specific configuration
│   ├── kustomization.yaml         # Kustomize aggregator
│   └── postgres/                  # PostgreSQL overlay
│       ├── kustomization.yaml     # Overlay configuration
│       ├── statefulset-patch.yaml # Resource limit overrides
│       └── secret-patch.yaml      # SOPS-encrypted credentials
├── cluster/magi/                  # FluxCD orchestration
│   ├── infrastructure.yaml        # Infrastructure Kustomization (Rook, Multus)
│   └── databases.yaml             # Database Kustomization (PostgreSQL)
└── .github/workflows/             # CI/CD automation
    ├── build-postgres-image.yml   # Container build and push
    └── check-postgres-updates.yml # Automated weekly version checks
```

## Next Steps

1. **Encrypt secrets**: Use SOPS to encrypt database password in overlay
2. **Test build workflow**: Push a test commit to trigger GitHub Actions
3. **Deploy to cluster**: Apply databases overlay via FluxCD
4. **Verify extensions**: Connect to database and verify pgvector/VectorChord loaded
5. **Monitor**: Watch logs for any startup issues or resource constraints
6. **Schedule backups**: Consider Velero snapshots for PVC backups

## Related Resources

- [PostgreSQL Official Docs](https://www.postgresql.org/docs/16/)
- [pgvector GitHub](https://github.com/pgvector/pgvector)
- [VectorChord GitHub](https://github.com/tensorchord/vectorchord)
- [FluxCD Documentation](https://fluxcd.io/docs/)
- [Kustomize Documentation](https://kustomize.io/)
- [SOPS Documentation](https://github.com/mozilla/sops)
