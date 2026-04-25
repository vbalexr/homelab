# Zigbee2MQTT - Home Lab Overlay

This overlay provides the **home** namespace-specific configuration for Zigbee2MQTT on the **magi** cluster.

## What This Overlay Provides

- **Namespace**: home
- **Persistence**: 1Gi PVC backed by Rook Ceph
- **Configuration**: Integration with Mosquitto MQTT broker

## Files in This Overlay

| File | Purpose | Type |
|------|---------|------|
| `kustomization.yaml` | Orchestrates resources and patches | Kustomize |
| `pvc.yaml` | 1Gi persistent storage for device database | Kubernetes PVC |
| `README.md` | This file | Documentation |

## Prerequisites

- Cluster must have `home` namespace
- Mosquitto MQTT broker must be running in the `home` namespace
- USB Zigbee adapter must be available on the host node (if using physical adapter)

## Quick Start

### 1. Customize Configuration

Edit the ConfigMap patch if needed for MQTT credentials or device settings.

### 2. Deploy

```bash
kubectl apply -k overlays/magi/home/zigbee2mqtt
```

### 3. Access Web UI

The web UI is available on `zigbee2mqtt:8080` within the cluster.

## MQTT Connection

By default, Zigbee2MQTT connects to the Mosquitto broker at `mosquitto:1883` on the MQTT base topic `zigbee2mqtt`.

To enable authentication, you can patch the ConfigMap with MQTT credentials:

```yaml
mqtt:
  user: z2m
  password: your-password
```
