# containers

Custom container images for applications that cannot use an off-the-shelf image directly.

Each subdirectory contains the `Dockerfile` and any supporting build context for a single image.

## Directory Layout

```
containers/
└── <app-name>/
    ├── Dockerfile
    └── ...          # Build context files
```

## Building an Image

```bash
docker build -t <registry>/<app-name>:<tag> containers/<app-name>/
```

## Guidelines

- Keep images minimal — use distroless or Alpine-based images where possible.
- Pin base image versions (avoid `latest`).
- Do **not** embed secrets or credentials inside images; use environment variables or mounted secrets at runtime.
