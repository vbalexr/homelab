# Vaultwarden

Self-hosted Bitwarden-compatible server.

## Notes

- Uses PostgreSQL (see `DATABASE_URL` in the overlay secret).
- Stores attachments and configuration under `/data`.
- Set `DOMAIN` to the public URL of the service.
- Provide a ConfigMap in the overlay to hold environment settings.

## Overlay ConfigMap Example

Create `overlays/<cluster>/home/vaultwarden/configmap.yaml`:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
	name: vaultwarden-config
	namespace: home
data:
	DOMAIN: "https://vault.example.com"
	SIGNUPS_ALLOWED: "false"
	INVITATIONS_ALLOWED: "true"
	WEBSOCKET_ENABLED: "true"
	ROCKET_PORT: "80"
	LOG_LEVEL: "info"
```

## Database Setup

Create a dedicated database and user in PostgreSQL:

```sql
CREATE USER home_vaultwarden WITH PASSWORD 'password';
CREATE DATABASE home_vaultwarden OWNER home_vaultwarden;
```

Update `DATABASE_URL` in the overlay secret accordingly.
