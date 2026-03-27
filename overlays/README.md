# overlays

Kustomize overlays that customise base application definitions for a specific cluster and namespace.

The directory hierarchy mirrors `cluster/<cluster-name>/<namespace>` so it is immediately clear which cluster and namespace an overlay targets.

## Directory Layout

```
overlays/
└── <cluster-name>/
    └── <namespace>/
        ├── kustomization.yaml   # References ../../apps/<app-name> as a base
        └── patch-*.yaml         # Patches applied on top of the base
```

## Adding a New Overlay

1. Create `overlays/<cluster>/<namespace>/kustomization.yaml`.
2. Set the `resources` list to point at the relevant `apps/<app-name>` base.
3. Add any patches needed for that specific environment.
