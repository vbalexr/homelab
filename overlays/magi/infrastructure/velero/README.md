# Velero for Magi Cluster

Magi-specific Velero deployment for backup solution with Backblaze B2 S3 backend storage.

This configuration includes:
- Complete Velero Helm chart deployment (v5.0.0)
- Backblaze B2 S3-compatible storage backend configuration
- Backblaze authentication credentials
- Daily and weekly backup schedules
- CSI volume snapshot support with Restic for pod volumes

## Configuration

- **Backend**: Backblaze B2 S3 (cloud-hosted, off-site backups)
- **S3 Endpoint**: `https://s3.us-west-004.backblazeb2.com` (adjust region as needed)
- **Bucket**: Your Backblaze B2 bucket name (configure in backblaze-secret.yaml)
- **Authentication**: Application Key ID + Application Key
- **Helm Chart Version**: 5.0.0 from vmware-tanzu/helm-charts
- **Daily Backup**: 02:00 UTC, 7-day retention
- **Weekly Backup**: Sundays 00:00 UTC, 14-day retention

## Files

- **kustomization.yaml** — Orchestrates Helm chart, namespace, schedules, and secrets
- **namespace.yaml** — Velero namespace definition
- **schedule.yaml** — Daily and weekly backup schedules
- **helm-values.yaml** — Helm chart values for Velero
- **backblaze-secret.yaml** — Backblaze B2 credentials (Application Key ID + Key)

## Setup Instructions

### 1. Create Backblaze B2 Bucket and Application Key

On your Backblaze B2 account:
1. Create a new bucket (e.g., `magi-velero-backups`)
2. Create an Application Key with access to this bucket:
   - Go to Backblaze > Account > Application Keys
   - Click "Create New Application Key"
   - Set permissions to **Read and Write**
   - Save the **Application Key ID** and **Application Key**

### 2. Configure Credentials

Edit `backblaze-secret.yaml` with your Backblaze credentials:

```yaml
stringData:
  cloud: |
    [default]
    aws_access_key_id=YOUR_BACKBLAZE_KEY_ID
    aws_secret_access_key=YOUR_BACKBLAZE_APPLICATION_KEY
```

### 3. Update Helm Values (if needed)

Edit `helm-values.yaml` to customize:

```yaml
configuration:
  backupStorageLocation:
    bucket: your-bucket-name  # Your Backblaze bucket
    config:
      s3Url: https://s3.us-west-004.backblazeb2.com  # Adjust region
```

Regional S3 endpoints for Backblaze B2:
- US West: `https://s3.us-west-004.backblazeb2.com`
- US West (Portland): `https://s3.us-west-001.backblazeb2.com`
- EU Central: `https://s3.eu-central-001.backblazeb2.com`
- EU: `https://s3.eu-000.backblazeb2.com`

### 4. Deploy

```bash
git add overlays/magi/infrastructure/velero/
git commit -m "feat: configure Velero with Backblaze B2 backend"
git push
```

Flux will deploy within 10 minutes.

## Verification

```bash
# Check Velero pod status
kubectl get pods -n velero

# Verify Backblaze secret is mounted
kubectl describe secret velero-credentials -n velero

# Check Velero logs for S3 connection
kubectl logs -n velero -l app.kubernetes.io/name=velero

# List backups
velero backup get

# Trigger test backup
velero backup create test-backup --wait
```

## Advantages of Backblaze B2 S3

✅ **Off-site storage** — Backups stored in cloud, not on-premise
✅ **Disaster recovery** — Protect against complete cluster loss
✅ **Cost-effective** — Backblaze B2 is cheaper than major cloud providers
✅ **S3-compatible** — Works with any S3-compatible tool
✅ **Scalable** — Unlimited storage (pay per GB)
✅ **Versioning** — Keep multiple versions of data

## Cost Estimate (Example)

For Magi cluster backups:
- **Storage**: ~$0.006/GB/month (B2 pricing)
- **Transactions**: Minimal read/write costs
- Example: 100 GB backups retained 7 days = ~$0.04/month

## To Deploy to Another Cluster

Create a new overlay similar to this one:

```
overlays/<cluster-name>/infrastructure/velero/
├── kustomization.yaml          (Helm chart + resources config)
├── namespace.yaml              (cluster namespace definition)
├── schedule.yaml               (backup schedules)
├── helm-values.yaml            (Helm values)
└── <storage>-secret.yaml       (cluster-specific credentials)
```

Copy this entire directory and update the secret file with different credentials per cluster. No shared base app - each overlay is self-contained.
