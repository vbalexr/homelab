# Mosquitto MQTT Broker

Lightweight MQTT message broker. Mosquitto 2.1.2 with init container for password setup.

Listeners: 1883 (MQTT), 9001 (MQTT-WS), 8883 (MQTT-TLS), 9002 (MQTT-WSS)

Runs as non-root (UID 1883). Environment-specific config via overlays.
  # Add pvc.yaml, certificate.yaml as needed
```

## Password Authentication

### Generate Hashed Passwords

Use the `mosquitto_passwd` CLI tool to generate hashed passwords for Mosquitto 2.1.2:

```bash
# Create a password file locally
mosquitto_passwd -c passwd.txt username
# You'll be prompted to enter and confirm the password

# Add additional users (without -c flag)
mosquitto_passwd passwd.txt another_user

# View the generated hashed passwords
cat passwd.txt
```

### Enable Authentication

1. **Generate passwords** using `mosquitto_passwd` (see above)

2. **Create a Secret in your overlay** (`secret-users.yaml`):
   ```yaml
   apiVersion: v1
   kind: Secret
   metadata:
     name: mosquitto-users
   type: Opaque
   stringData:
     passwd: |
       ---- passwd.txt content here ----
     acl: |
       user hassio
       topic readwrite #
       
       user z2m
       topic readwrite zigbee2mqtt/#
   ```
   
   Replace the hashed passwords with output from your `mosquitto_passwd` command.

3. **Update configmap-mosquitto.yaml** (in the base app):
   - Uncomment: `password_file /mosquitto/data/passwd`
   - Uncomment: `acl_file /mosquitto/data/acl` (if using ACLs)
   - Change: `allow_anonymous false` (in each listener)

4. **Add the Secret to your overlay's kustomization.yaml**:
   ```yaml
   resources:
     - ../../../../apps/mosquitto
     - secret-users.yaml
   ```

5. **Redeploy**:
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
