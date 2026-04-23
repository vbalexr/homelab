# Mosquitto MQTT Broker

Eclipse Mosquitto is a lightweight open-source message broker that implements the MQTT protocol.

This is the **base application** that provides the core Mosquitto deployment. Environment-specific configuration (PVC size, TLS certificates, credentials) should be provided by overlays.

## What This App Provides

- **Deployment**: Mosquitto 2.1.2 container with init container for password setup
- **Service**: Exposes 4 protocol listeners (MQTT, MQTT-WS, MQTT-TLS, MQTT-WSS)
- **ConfigMap**: Main mosquitto.conf configuration file
- **Init Container**: Copies password files from Kubernetes Secret to container filesystem

## Listeners

The app configures and listens on 4 ports:

| Port | Protocol | Default | Notes |
|------|----------|---------|-------|
| 1883 | MQTT | `allow_anonymous true` | Plain MQTT protocol |
| 9001 | MQTT-WS | `allow_anonymous true` | WebSocket transport |
| 8883 | MQTT-TLS | Disabled | TLS/SSL (enable in overlay) |
| 9002 | MQTT-WSS | Disabled | WebSocket + TLS (enable in overlay) |

All listeners are configured but can be enabled/disabled via the ConfigMap.

## Configuration

### mosquitto.conf (ConfigMap)

The main configuration includes:

- **Persistence**: Enabled, stores to `/mosquitto/data/`
- **Logging**: Logs to stdout for container log capture
- **Password Authentication**: Commented by default, uncomment and provide hashed passwords via overlay
- **ACL Support**: Configured to read from `/mosquitto/data/acl` when enabled

### Volumes

The deployment expects these volume mounts:

| Mount Point | Type | Expected Source | Notes |
|-------------|------|-----------------|-------|
| `/mosquitto/config` | ConfigMap | `mosquitto-config` | Main config file (read-only) |
| `/mosquitto/data` | PVC or emptyDir | Provided by overlay | Stores logs, messages, password file |
| `/mosquitto/certs` | Secret | Provided by overlay (optional) | TLS certificates, if enabled |

## Init Container

The `setup-passwords` init container runs before Mosquitto starts:

1. **Source**: Reads hashed passwords from the `mosquitto-users` Secret
2. **Destination**: Copies `passwd` and `acl` files to `/mosquitto/data/`
3. **Permissions**: Sets proper file modes for security
4. **Benefit**: Allows credentials to be stored as Kubernetes Secrets while persisting in the mounted volume

## Security Context & Pod Security Policy

### Main Container (mosquitto)

The main Mosquitto container is **fully compliant** with Kubernetes `restricted:latest` Pod Security policy:

- Runs as non-root user (UID 1883, mosquitto)
- `allowPrivilegeEscalation: false`
- Capabilities dropped: `["ALL"]`
- `seccompProfile: RuntimeDefault`

### Init Container (setup-passwords)

The init container requires root access for system setup operations (copying files, setting permissions). **PodSecurity warnings for the init container are expected and acceptable**:

- Runs as UID 0 (root) to copy files and set permissions
- This is a documented pattern for system setup in Kubernetes
- The main application container remains fully restricted

This approach provides the best balance between security (restrictive main container) and functionality (system setup when needed).

## Usage with Overlays

This base app is designed to be used with overlays for environment-specific configuration:

```
overlays/<cluster>/<namespace>/mosquitto/
  ├── kustomization.yaml       # Assembles resources
  ├── pvc.yaml                 # (optional) PVC for /mosquitto/data
  ├── certificate.yaml         # (optional) TLS cert via cert-manager
  ├── secret-users.yaml        # Password file (from generate-passwords.py)
  └── deployment-patch.yaml    # (optional) Additional patches
```

### Minimal Overlay Example

```yaml
# kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  - ../../../../apps/mosquitto
  - secret-users.yaml
  # Add pvc.yaml, certificate.yaml as needed
```

## Password Authentication

### Generate Hashed Passwords

Use the provided script to generate argon2id hashed passwords compatible with Mosquitto 2.1.2:

```bash
./generate-passwords.py
```

This outputs:
- Base64-encoded passwd file (copy to your Secret resource)
- Plain text for reference
- ACL rules (if using the default users)

### Enable Authentication

1. **Generate passwords**:
   ```bash
   ./generate-passwords.py
   ```

2. **Update your overlay's Secret**:
   ```yaml
   apiVersion: v1
   kind: Secret
   metadata:
     name: mosquitto-users
   type: Opaque
   data:
     passwd: <base64-output-from-script>
     acl: <base64-encoded-acl-rules>
   ```

3. **Update configmap-mosquitto.yaml** (this file):
   - Uncomment: `password_file /mosquitto/data/passwd`
   - Uncomment: `acl_file /mosquitto/data/acl`
   - Change: `allow_anonymous false` (in each listener)

4. **Redeploy**:
   ```bash
   kubectl apply -k <your-overlay>
   ```

## Customization

### Change Mosquitto Version

Edit `deployment.yaml`:

```yaml
containers:
  - name: mosquitto
    image: eclipse-mosquitto:2.1.2  # Change version here
```

### Add Custom Listeners

Edit `configmap-mosquitto.yaml` to add, modify, or remove listeners. All listeners support these options:

- `allow_anonymous true/false`
- `password_file <path>` (when auth enabled)
- `acl_file <path>` (when auth enabled)

### Enable TLS

Uncomment the TLS listeners in `configmap-mosquitto.yaml`:

```yaml
listener 8883
protocol mqtt
certfile /mosquitto/certs/tls.crt
keyfile /mosquitto/certs/tls.key

listener 9002
protocol websockets
certfile /mosquitto/certs/tls.crt
keyfile /mosquitto/certs/tls.key
```

Your overlay should provide the `/mosquitto/certs` secret with TLS certificates.

## Testing

### Test Anonymous Access

```bash
# Subscribe
mosquitto_sub -h mosquitto -p 1883 -t test/#

# Publish (in another terminal)
mosquitto_pub -h mosquitto -p 1883 -t test/msg -m "hello"
```

### Test with Authentication (after setup)

```bash
mosquitto_pub -h mosquitto -p 1883 \
  -u <username> -P <password> \
  -t test/msg -m "hello"
```

### Test WebSocket

Use a WebSocket MQTT client with URL: `ws://mosquitto:9001`

## Files

- `deployment.yaml` - Mosquitto deployment with init container
- `service.yaml` - 4-port Service resource
- `configmap-mosquitto.yaml` - mosquitto.conf configuration
- `kustomization.yaml` - Base kustomization
- `generate-passwords.py` - Script to generate hashed passwords

## Notes

- The init container requires the `mosquitto-users` Secret to exist (created by overlay)
- All persistence should be configured via overlay (PVC mount, size, storage class)
- TLS certificates should be provided by overlay (via Secret or cert-manager)
- This app works with any namespace and storage backend

## Usage

In the overlay (e.g., `overlays/magi/home/mosquitto/`), create `secret-users.yaml`:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: mosquitto-users
  namespace: home
type: Opaque
stringData:
  passwd: |
    user1:password1
    user2:password2

  acl: |
    user admin
    topic readwrite #
    
    user user1
    topic read sensors/#
    topic write commands/user1/#
```

Then add to overlay's `kustomization.yaml`:
```yaml
resources:
  - ../../../../apps/mosquitto
  - secret-users.yaml
```

## Security Notes

- User credentials and ACL are stored in Kubernetes Secrets (provided by overlay)
- Secrets should be encrypted with SOPS in production
- Passwords should be hashed using `mosquitto_passwd` tool
- Use RBAC to restrict access to the secret
- Enable TLS/SSL via cert-manager (configured in overlay)

## Ports

- **1883**: MQTT (plain, unencrypted)
- **8883**: MQTT over TLS (encrypted)
- **9001**: MQTT over WebSocket (plain, unencrypted)
- **9002**: MQTT over Secure WebSocket (encrypted with TLS)

## Accessing Mosquitto

From within the cluster:
```bash
mosquitto_pub -h mosquitto -t "test/topic" -m "Hello"
mosquitto_sub -h mosquitto -t "test/#"
```
