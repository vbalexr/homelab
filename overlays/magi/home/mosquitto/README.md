# Mosquitto MQTT Broker - Home Lab Overlay

Home namespace-specific Mosquitto config on magi cluster.

- Namespace: home
- Storage: 10Gi PVC (Rook Ceph)
- TLS: Auto-renewing via cert-manager
- Auth: Password + ACL

```bash
kubectl apply -k overlays/magi/home/mosquitto/
```
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
