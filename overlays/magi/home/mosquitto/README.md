# Mosquitto MQTT Broker - Home Lab Overlay

This overlay provides the **home** namespace-specific configuration for Mosquitto MQTT broker on the **magi** cluster.

## What This Overlay Provides

- **Namespace**: home
- **Persistence**: 10Gi PVC backed by Rook Ceph
- **TLS/SSL**: Auto-renewing certificates via cert-manager and Let's Encrypt
- **Authentication**: Password-based access control with ACL rules
- **Users**: 3 pre-configured users (hassio, z2m, device)

## Files in This Overlay

| File | Purpose | Type |
|------|---------|------|
| `kustomization.yaml` | Orchestrates resources and patches | Kustomize |
| `certificate.yaml` | TLS certificate via cert-manager | cert-manager |
| `secret-users.yaml` | Hashed passwords and ACL rules | Kubernetes Secret |
| `pvc.yaml` | 10Gi persistent storage | Kubernetes PVC |
| `configmap-mosquitto-patch.yaml` | Strategic merge patch for mosquitto.conf | ConfigMap patch |
| `.gitignore` | Protects plain-text password files | .gitignore |
| `passwords.txt` | Plain-text input (DO NOT COMMIT) | Local file |

## Quick Start

### 1. Prerequisites

- Cluster must have `home` namespace
- cert-manager v1+ installed with valid ClusterIssuer named `letsencrypt-prod`
- Rook Ceph storage provisioner available
- Base Mosquitto app at `apps/mosquitto/`

### 2. Customize Domain

Edit `certificate.yaml`:

```yaml
dnsNames:
  - mosquitto.example.com  # Change to your domain
```

# 4. Cleanup
```

### 4. Deploy

```bash
cd /home/akaki/Dev/vbalex/homelab
kubectl apply -k overlays/magi/home/mosquitto/
```

### 5. Verify

```bash
# Check pod is running
kubectl -n home get pod -l app=mosquitto

# Check logs
kubectl -n home logs -l app=mosquitto

# Check certificate is ready
kubectl -n home get certificate mosquitto-tls

# Verify TLS volume is mounted
kubectl -n home exec -it deployment/mosquitto -- ls -la /mosquitto/certs/
```

## Architecture Details

### Listeners

All 4 listeners are **enabled** in this overlay with **authentication required**:

| Port | Protocol | Auth | TLS | Status |
|------|----------|------|-----|--------|
| 1883 | MQTT | ✓ Password | ✗ | Active |
| 9001 | MQTT-WS | ✓ Password | ✗ | Active |
| 8883 | MQTT-TLS | ✓ Password | ✓ | Active |
| 9002 | MQTT-WSS | ✓ Password | ✓ | Active |

### Storage

- **Type**: ReadWriteOnce PVC (Rook Ceph backend)
- **Size**: 10Gi
- **Mount**: `/mosquitto/data`
- **Contents**: Logs, retained messages, password file, ACL file

### TLS Certificates

- **Provider**: cert-manager with Let's Encrypt
- **Challenge**: DNS01 via Cloudflare
- **Duration**: 90 days
- **Auto-Renew**: 30 days before expiry
- **Secret**: `mosquitto-tls` (created by cert-manager)
- **Mount**: `/mosquitto/certs` (read-only)

### Authentication

- **Passwords**: Argon2id hashed ($7$ format, Mosquitto 2.1.2 native format)
- **ACL**: Topic-based access control
- **Method**: Kubernetes Secret + init container pattern

#### Default Users

| Username | Permissions |
|----------|-------------|
| `hassio` | All topics (read/write) |
| `z2m` | zigbee2mqtt/* (read/write) |
| `device` | commands/device/* (read), sensors/device/* (write) |

## Kustomize Patching

### 1. ConfigMap Strategic Merge Patch

**File**: `configmap-mosquitto-patch.yaml`

Enables all 4 listeners, enforces authentication, and points to TLS certificates:

```yaml
# listener 8883, 9002 enabled
# allow_anonymous: false on all listeners
# password_file /mosquitto/data/passwd
# acl_file /mosquitto/data/acl
# TLS cert paths configured
```

### 2. Deployment JSON Patches

**Inline in**: `kustomization.yaml`

Adds mosquitto-tls volume and volumeMount:

```yaml
- op: add
  path: /spec/template/spec/volumes/-
  value: { mosquitto-tls Secret }
- op: add
  path: /spec/template/spec/containers/0/volumeMounts/-
  value: { /mosquitto/certs mount }
```

This pattern avoids resource duplication while extending the base deployment.

## Resource Dependencies

```
PVC (mosquitto-data)
  ↓
Deployment (depends on: ConfigMap, PVC, Secret)
  ├─ Init Container (setup-passwords)
  │   ├─ Source: Secret (mosquitto-users)
  │   └─ Dest: PVC (passwd, acl files)
  └─ Main Container (mosquitto)
      ├─ Config: ConfigMap
      ├─ Data: PVC
      ├─ Certs: Secret (mosquitto-tls)
      └─ Ports: Service (ClusterIP)

Certificate (mosquitto-tls)
  ↓
Secret (mosquitto-tls, auto-created by cert-manager)
  ↓
Deployment (volumeMount)
```

## Troubleshooting

### Pod not starting

```bash
kubectl -n home describe pod -l app=mosquitto
kubectl -n home logs -l app=mosquitto --previous
```

### Certificate not ready

```bash
kubectl -n home describe certificate mosquitto-tls
kubectl -n home get events | grep mosquitto
```

### Password authentication failing

```bash
# Check password file was copied
kubectl -n home exec deployment/mosquitto -- cat /mosquitto/data/passwd

# Check ACL file
kubectl -n home exec deployment/mosquitto -- cat /mosquitto/data/acl

# Check logs for auth errors
kubectl -n home logs -l app=mosquitto | grep -i "auth\|password\|acl"
```

### TLS connection failing

```bash
# Verify certificate is present
kubectl -n home exec deployment/mosquitto -- ls -la /mosquitto/certs/

# Check certificate validity
kubectl -n home get secret mosquitto-tls -o yaml | grep tls.crt | base64 -d | openssl x509 -text -noout
```

## Security Notes

1. **Plain-text passwords**: Keep `passwords.txt` in `.gitignore`; never commit
2. **Secret values**: Use Git-crypt or Sealed Secrets for production
3. **ACL enforcement**: `allow_anonymous false` on all listeners
4. **TLS**: All listeners have TLS available; WS/WSS recommended for web clients
5. **PVC permissions**: Init container sets restrictive perms (600 passwd, 700 acl)

## Updating Credentials

### Change a password

```bash
# 1. Edit passwords.txt with new password
# 2. Run generation script
python3 ../../../../apps/mosquitto/generate-passwords.py passwords.txt
# 3. Update secret-users.yaml data.passwd with base64 output
# 4. Apply and restart
kubectl apply -k overlays/magi/home/mosquitto/
kubectl -n home rollout restart deployment/mosquitto
```

## Cluster Integration

This overlay is designed for the **magi** cluster with:

- **Namespace**: home
- **Labels**: app.kubernetes.io/instance: magi, app.kubernetes.io/name: mosquitto
- **Storage**: Rook Ceph (matches cluster infrastructure)
- **DNS**: mosquitto.vbalex.com (or customized in certificate.yaml)

## Related Resources

- **Base Application**: [apps/mosquitto/README.md](../../../../apps/mosquitto/README.md)
- **cert-manager**: https://cert-manager.io/docs/
- **Mosquitto**: https://mosquitto.org/man/mosquitto-conf-5.html
- **Argon2id**: https://tools.ietf.org/html/draft-irtf-cfrg-argon2
