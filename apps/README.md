# apps

Base application definitions (environment-agnostic Kubernetes manifests).

Each app contains `kustomization.yaml` + manifests (Deployment, Service, ConfigMap, etc.).

Customize per-cluster using overlays in `overlays/<cluster>/<app>/`.
