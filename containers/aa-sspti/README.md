# aa-sspti container images

Container build context for the aa-sspti AllianceAuth stack.

Expected custom image names:
- `ghcr.io/vbalexr/aa-sspti-gunicorn`
- `ghcr.io/vbalexr/aa-sspti-beat`
- `ghcr.io/vbalexr/aa-sspti-worker`
- `ghcr.io/vbalexr/aa-sspti-worker_services`
- `ghcr.io/vbalexr/aa-sspti-nginx` (optional if you choose a custom nginx image)

Build examples:

```bash
docker build -f containers/aa-sspti/custom.dockerfile \
  --build-arg AA_DOCKER_TAG=registry.gitlab.com/allianceauth/allianceauth/auth:v4.13.1 \
  -t ghcr.io/vbalexr/aa-sspti-gunicorn:latest \
  containers/aa-sspti/

docker build -f containers/aa-sspti/custom.dockerfile \
  --build-arg AA_DOCKER_TAG=registry.gitlab.com/allianceauth/allianceauth/auth:v4.13.1 \
  -t ghcr.io/vbalexr/aa-sspti-worker:latest \
  containers/aa-sspti/

docker build -f containers/aa-sspti/custom.dockerfile \
  --build-arg AA_DOCKER_TAG=registry.gitlab.com/allianceauth/allianceauth/auth:v4.13.1 \
  -t ghcr.io/vbalexr/aa-sspti-worker_services:latest \
  containers/aa-sspti/

docker build -f containers/aa-sspti/custom.dockerfile \
  --build-arg AA_DOCKER_TAG=registry.gitlab.com/allianceauth/allianceauth/auth:v4.13.1 \
  -t ghcr.io/vbalexr/aa-sspti-beat:latest \
  containers/aa-sspti/

```

Note: the Kubernetes manifests can also use `docker.io/nginx:stable` directly for nginx.
