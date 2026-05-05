# Frigate

Frigate NVR with real-time AI object detection.

Base application definition (environment-agnostic Kubernetes manifests).

## Features

- Deployment with 1 replica
- Service exposing UI and RTSP/WebRTC ports
- Persistent storage for config and media
- Health checks (liveness and readiness probes)
- Non-privileged capabilities and default seccomp profile

## Customization

Customize per-cluster using overlays in `overlays/<cluster>/home/frigate/`.
