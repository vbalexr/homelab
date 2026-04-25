# Zigbee2MQTT Bridge

Zigbee to MQTT gateway for home automation. Web UI on port 8080.

Requires configuration file at `/app/data/configuration.yaml`. Runs as non-root.

Environment-specific config (USB device, MQTT credentials) via overlays.

This base app is designed to be used with overlays for environment-specific configuration:

```
overlays/<cluster>/<namespace>/zigbee2mqtt/
  ├── kustomization.yaml       # Assembles resources
  ├── pvc.yaml                 # PVC for /app/data
  └── deployment-patch.yaml    # (optional) Device patches
```

### Minimal Overlay Example

```yaml
# kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  - ../../../../apps/zigbee2mqtt
  - pvc.yaml
```
