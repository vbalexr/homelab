# apps

Base application definitions (environment-agnostic Kubernetes manifests).

Each app directory contains a `kustomization.yaml` + manifests (Deployment, Service, ConfigMap, etc.).

## Adding an App

```bash
mkdir apps/my-app
cd apps/my-app
cat > kustomization.yaml << EOF
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - deployment.yaml
  - service.yaml
EOF
```

## Customization

Use overlays in `overlays/<cluster>/<app>/` for environment-specific changes.
