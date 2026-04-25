# Zigbee2MQTT Bridge

Zigbee2MQTT is a Zigbee to MQTT gateway that bridges Zigbee devices to MQTT for home automation.

This is the **base application** that provides the core Zigbee2MQTT deployment. Environment-specific configuration (USB device mapping, MQTT credentials, certificate settings) should be provided by overlays.

## What This App Provides

- **Deployment**: Zigbee2MQTT container with support for USB device passthrough
- **Service**: Exposes the web UI (port 8080) and MQTT connection
- **ConfigMap**: Main configuration.yaml file for Zigbee2MQTT settings

## Ports

The app exposes the following port:

| Port | Protocol | Purpose | Notes |
|------|----------|---------|-------|
| 8080 | HTTP | Web UI | For manual device management and configuration |


## Configuration

### configuration.yaml

Zigbee2MQTT requires a configuration file at `/app/data/configuration.yaml`. The init container will wait for this file to be created before starting the application.

**Setup Steps**:

1. Deploy the application (pod will wait in init container):
   ```bash
   kubectl apply -k overlays/magi/home/zigbee2mqtt
   ```

2. Create a configuration file locally (see example below)

3. Copy the configuration to the running pod:
   ```bash
   kubectl cp ./configuration.yaml <pod-name>:/app/data/configuration.yaml -n home
   ```

4. The application will start automatically once the file is detected

**Configuration Template**:

Example `configuration.yaml` to use with `kubectl cp`. This connects to Mosquitto broker:

```yaml
homeassistant: false

mqtt:
  base_topic: zigbee2mqtt
  server: mqtt://mosquitto:1883  # Connects to Mosquitto broker
  # user: z2m  # Uncomment if using authentication
  # password: password

serial:
  port: /dev/ttyUSB0
  baudrate: 115200
  rtscts: true

frontend:
  port: 8080

advanced:
  legacy_api: false
  pan_id: 6754
  channel: 11

permit_join: false
log_level: info
```

- **Homeassistant**: Integration with Home Assistant (disabled by default)
- **MQTT**: Connection settings to MQTT broker
- **Serial**: USB serial device configuration (must be provided by overlay)
- **Advanced**: Various advanced settings for device handling

### Volumes

The deployment uses the following volume mounts:

| Mount Point | Type | Source | Notes |
|-------------|------|--------|-------|
| `/app/data` | PVC | Persistent volume claim | Stores device database, configuration, and logs |

The init container ensures the configuration file exists and is writable for the application.

## Security Context & Pod Security Policy

The Zigbee2MQTT container requires:

- Runs as non-root user (node user)
- `allowPrivilegeEscalation: false`
- Capabilities dropped: `["ALL"]`

Note: USB device access may require special node affinity and device plugin configuration.

## Usage with Overlays

This base app is designed to be used with overlays for environment-specific configuration:

```
overlays/<cluster>/<namespace>/zigbee2mqtt/
  ├── kustomization.yaml       # Assembles resources
  ├── pvc.yaml                 # PVC for /app/data
  └── deployment-patch.yaml    # (optional) Device patches
```

### Minimal Overlay Example

```yaml
# kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  - ../../../../apps/zigbee2mqtt
  - pvc.yaml
```
