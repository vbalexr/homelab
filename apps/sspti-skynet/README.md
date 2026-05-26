# SSPTI Skynet

AI-powered Discord bot for EVE Online corporations.

## Configuration

### Environment Variables

**Required Secrets (stored in Secret):**
- `DISCORD_TOKEN` - Discord bot token from [Discord Developer Portal](https://discord.com/developers/applications)
- `JANICE_API_KEY` - API key from [Janice Appraisal](https://janice.e-351.com)
- `OPENAI_API_KEY` - OpenAI or compatible LLM API key
- `ESI_USER_AGENT` - User agent for ESI requests (format: `Skynet/0.1 (contact: your-email@example.com)`)
- `JANICE_BASE_URL` - Janice API endpoint
- `OPENAI_BASE_URL` - LLM endpoint
- `OPENAI_MODEL` - LLM model name
- `OPENAI_MAX_TOOL_HOPS` - Max tool calls per request
- `OPENAI_MAX_REFINEMENT_HOPS` - Max refinement iterations
- `DATABASE_URL` - PostgreSQL connection string

**Non-sensitive Configuration (stored in ConfigMap):**
- `ESI_BASE_URL` - EVE Swagger Interface endpoint (default: `https://esi.evetech.net/latest`)
- `ESI_DEFAULT_REGION_ID` - Default region ID (default: `10000002` = The Forge)
- `LOG_LEVEL` - Logging level (default: `INFO`)
- `corp.yaml` - Corporation configuration file content (mounted to `/app/corp.yaml`)

### Corporation Config File

The bot requires a `corp.yaml` configuration file mounted at `/app/corp.yaml`. Example structure:

```yaml
corporation: "Your Corp Name"
ticker: "TICK"
division_id: 12345
regions:
  - region_id: 10000002
    name: "The Forge"
    station_ids:
      - 60003760
  - region_id: 10000043
    name: "Domain"
divisions:
  logistics:
    discord_channel_id: 1234567890
    manager_role_id: 1234567890
```

## Kubernetes Deployment Guide

### Prerequisites

- Kubernetes cluster with FluxCD and SOPS installed
- PostgreSQL database accessible to the cluster
- Discord bot token
- Janice API key
- LLM endpoint (OpenAI, Ollama, etc.)

### Deployment Steps

#### 1. Prepare Configuration Files

Copy example files from `apps/sspti-skynet/`:

```bash
cd overlays/magi/sspti/

# Copy and edit Secret example
cp ../../apps/sspti-skynet/secret.yaml.example sspti-skynet-secret.yaml
# Edit with your actual values (tokens, keys, URLs)
nano sspti-skynet-secret.yaml

# Copy and edit ConfigMap example (includes corp.yaml)
cp ../../apps/sspti-skynet/configmap.yaml.example sspti-skynet-configmap.yaml
# Edit with your corporation details in the corp.yaml section
nano sspti-skynet-configmap.yaml
```

The Secret must be encrypted with SOPS before committing:

```bash
# Edit the secret with SOPS (will auto-encrypt on save)
sops sspti-skynet-secret.yaml

# Or encrypt an existing file
sops -e -i sspti-skynet-secret.yaml

# Verify it's encrypted
cat sspti-skynet-secret.yaml | head -20
# Should show: sops:, age:, ENC[...]
```

#### 3. Update Kustomization

Verify `kustomization.yaml` includes all resources:

```yaml
resources:
  - namespace.yaml
  - ../../../apps/aa-sspti
  - ../../../apps/sspti-skynet
  - secret.yaml
  - configmap.yaml
  - sspti-skynet-secret.yaml
  - sspti-skynet-configmap.yaml
  - aa-sspti-image-automation.yaml

images:
  - name: ghcr.io/mrakaki/sspti-skynet
    newTag: 20260526-2216 # {"$imagepolicy":"sspti:sspti-skynet:tag"}
```

#### 4. Deploy

Commit and push to main branch:

```bash
git add overlays/magi/sspti/sspti-skynet-*.yaml
git commit -m "chore: configure sspti-skynet"
git push
```

FluxCD will automatically deploy within 1 minute.

#### 6. Verify Deployment

```bash
# Check pod status
kubectl get pods -n sspti -l app=sspti-skynet

# View logs
kubectl logs -n sspti -l app=sspti-skynet -f

# Verify environment variables
kubectl exec -n sspti deployment/sspti-skynet -- env | grep DISCORD
```

### Configuration Files Reference

| File | Location | Purpose | Encryption |
|------|----------|---------|-----------|
| `secret.yaml.example` | `apps/sspti-skynet/` | Template for sensitive config | Reference only |
| `configmap.yaml.example` | `apps/sspti-skynet/` | Template for non-sensitive config | Reference only |
| `corp.yaml.example` | `apps/sspti-skynet/` | Template for corpoenvironment vars + corp.yaml | Reference only |
| `corp.yaml.example` | `apps/sspti-skynet/` | Reference corp.yaml structure | Reference only |
| `sspti-skynet-secret.yaml` | `overlays/magi/sspti/` | **Actual secret** | **MUST encrypt with SOPS** |
| `sspti-skynet-configmap.yaml` | `overlays/magi/sspti/` | **Environment vars + corp.yaml**
### Troubleshooting

**Secret not decrypted by FluxCD:**
```bash
# Check SOPS configuration
cat .sops.yaml

# Verify secret is encrypted
grep "ENC\[" overlays/magi/sspti/sspti-skynet-secret.yaml

# Check FluxCD can decrypt
flux logs --all-namespaces | grep sspti
```

**Pod stuck in pending:**
```bash
# Check pod events
kubectl describe pod -n sspti -l app=sspti-skynet

# Check if ConfigMap/Secret exist
kubectl get secret,cm -n sspti -l app=sspti-skynet
```

**Environment variables not injected:**
```bash
# Verify Secret/ConfigMap content
kubectl get secret sspti-skynet-secret -n sspti -o jsonpath='{.data}' | jq .

# Check deployment spec
kubectl get deployment sspti-skynet -n sspti -o yaml | grep -A 10 envFrom
```

## Image Updates

Image updates are automated via FluxCD ImagePolicy. The image tag in `kustomization.yaml` is automatically updated when new images are available matching the pattern `YYYYMMDD-HHMM`.
