# overlays

Kustomize overlays for cluster-specific customizations.

```
overlays/
└── <cluster>/
    └── <namespace>/
        ├── kustomization.yaml    # References apps/<app>
        └── *-patch.yaml          # Environment-specific patches
```

## Usage

```bash
kubectl apply -k overlays/magi/databases/
```
