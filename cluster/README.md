# cluster

Per-cluster Kubernetes configurations and FluxCD bootstrap.

Directory: `cluster/<cluster-name>/` contains flux-system/, databases.yaml, infrastructure.yaml, and talos/

## Bootstrap FluxCD

```bash
flux bootstrap github --owner=<owner> --repository=homelab --branch=main --path=cluster/<cluster-name>
```

See [Talos setup](magi/talos/README.md).
# Edit /tmp/secret.yaml
sops --encrypt /tmp/secret.yaml > cluster/<cluster-name>/velero-secret.enc.yaml
rm /tmp/secret.yaml
```

### 8. Verify Encryption

```bash
# Check file is encrypted (will show binary/garbled content)
cat cluster/<cluster-name>/velero-secret.enc.yaml | head

# Decrypt and view
sops --decrypt cluster/<cluster-name>/velero-secret.enc.yaml

# Verify Flux can decrypt
kubectl get secrets -n velero
```

### Example: Encrypt Backblaze Credentials

```bash
# Reference: overlays/magi/infrastructure/velero/backblaze-secret.yaml

# 1. Create unencrypted
cat > backblaze-secret-plain.yaml << EOF
apiVersion: v1
kind: Secret
metadata:
  name: velero-credentials
  namespace: velero
type: Opaque
stringData:
  cloud: |
    [default]
    aws_access_key_id=004f44aeec97b540000000003
    aws_secret_access_key=K004O4L2UXVsiMo6V+hvpY8CNoghrzQ
EOF

# 2. Encrypt with SOPS
sops --encrypt backblaze-secret-plain.yaml > overlays/magi/infrastructure/velero/backblaze-secret.enc.yaml

# 3. Update kustomization.yaml to reference encrypted secret
# overlays/magi/infrastructure/velero/kustomization.yaml:
# resources:
#   - ../../../apps/velero
#   - backblaze-secret.enc.yaml

# 4. Commit encrypted version
git add overlays/magi/infrastructure/velero/backblaze-secret.enc.yaml
git rm backblaze-secret-plain.yaml  # Remove plain text

# 5. Flux will automatically decrypt via cluster/magi/infrastructure.yaml
#    (decryption is configured at the parent Kustomization level)
```

## Troubleshooting SOPS

### "Error: age: failed to decrypt data"

The age key isn't available. Check:
```bash
# Verify secret exists in cluster
kubectl get secrets -n flux-system sops-age

# Check Flux logs
kubectl logs -n flux-system deployment/source-controller
kubectl logs -n flux-system deployment/kustomize-controller
```

### "sops: command not found"

Install SOPS:
```bash
# Verify installation
which sops

# Or reinstall
brew reinstall sops  # macOS
```

### Decrypt fails locally

Wrong age key or no key configured:
```bash
# Set age key path
export SOPS_AGE_KEY_FILE=cluster/<cluster-name>/.sops.age
sops --decrypt cluster/<cluster-name>/velero-secret.enc.yaml
```

### Re-encrypt with different key

If you rotate keys:
```bash
# Decrypt with old key
SOPS_AGE_KEY_FILE=old.age sops --decrypt secret.enc.yaml > secret-plain.yaml

# Encrypt with new key
SOPS_AGE_KEY_FILE=new.age sops --encrypt secret-plain.yaml > secret.enc.yaml

# Update Flux secret
kubectl create secret generic sops-age \
  --from-file=age.agekey=new.age \
  -n flux-system \
  --dry-run=client -o yaml | kubectl apply -f -
```

## References

- [SOPS Documentation](https://github.com/mozilla/sops)
- [Age Documentation](https://github.com/FiloSottile/age)
- [Flux Secrets Documentation](https://fluxcd.io/docs/guides/mozilla-sops/)
- [GitOps Best Practices](https://kustomize.io/)
