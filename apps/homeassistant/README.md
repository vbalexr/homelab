# Home Assistant

Home Assistant - Open source home automation.

Base application definition (environment-agnostic Kubernetes manifests).

## Features

- Deployment with 1 replica
- Service on port 8123
- Persistent storage for configuration
- Health checks (liveness and readiness probes)
- Resource requests and limits
- Non-root security context

## Customization

Customize per-cluster using overlays in `overlays/<cluster>/home/homeassistant/`.
