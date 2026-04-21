# containers

Custom Docker images for applications requiring specialized builds.

```
containers/
└── <app-name>/
    ├── Dockerfile
    └── ...
```

## Build

```bash
docker build -t <registry>/<app>:<tag> containers/<app>/
```

## Guidelines

- Use minimal base images (Alpine, distroless)
- Pin versions (avoid `latest`)
- No secrets in images
