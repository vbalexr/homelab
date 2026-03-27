# apps

Base application definitions shared across clusters.

Each subdirectory contains a Kustomize `kustomization.yaml` along with Kubernetes manifests (Deployments, Services, ConfigMaps, etc.) for a single application.

These base manifests are environment-agnostic. Cluster- and namespace-specific configuration is applied via overlays in the `overlays/` directory.

## Adding a New Application

```
apps/
└── my-app/
    ├── kustomization.yaml
    ├── deployment.yaml
    ├── service.yaml
    └── configmap.yaml
```
