# aa-sspti container images

Container build context for the aa-sspti AllianceAuth stack.

## Versioning

The AllianceAuth version is defined in a single file:

```
containers/aa-sspti/VERSION
```

To upgrade, update that file and commit. The CI workflow reads it automatically and passes it as a build arg to all images.

## Published images

- `ghcr.io/vbalexr/aa-sspti`
- `ghcr.io/vbalexr/aa-sspti-nginx` (optional if you choose a custom nginx image)

## Local build examples

Read the version and pass it as a build arg:

```bash
AUTH_VERSION=$(cat containers/aa-sspti/VERSION | tr -d '[:space:]')

docker build -f containers/aa-sspti/custom.dockerfile \
  --build-arg AUTH_VERSION=$AUTH_VERSION \
  -t ghcr.io/vbalexr/aa-sspti:latest \
  containers/aa-sspti/
```

Note: the Kubernetes manifests can also use `docker.io/nginx:stable` directly for nginx.
