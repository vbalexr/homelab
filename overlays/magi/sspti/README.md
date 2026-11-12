# sspti

AllianceAuth, reached through the Cloudflare tunnel (straight to the `aa-sspti-web` Service).

- **Image:** `ghcr.io/vbalexr/aa-sspti` (pinned by digest), built by the `Build aa-sspti Images` workflow from `containers/aa-sspti`.
  `conf/constraints.txt` is the `pip freeze` of the image and pins every Python dependency. The package is private: pods use the `ghcr-pull` Secret.
- **Pods:** `aa-sspti-web` (gunicorn + nginx; `collectstatic` fills an `emptyDir` shared with nginx), `aa-sspti-worker`, `aa-sspti-worker-services`,
  `aa-sspti-beat` (exactly one), `aa-sspti-discordbot` (exactly one). The configuration (`local.py`, `celery.py`, `urls.py`) comes from the
  `aa-sspti-config` ConfigMap, the environment from `aa-sspti-env` and `aa-sspti-secret`.
- **Start command:** every container runs `pip install django-health-check==4.8.0` first (`urls.py` imports it), so the pods need outbound internet.
- **Database:** MariaDB `sspti_aa` (account `sspti_aa`) in the `database` namespace, reached through the `mariadb` ExternalName Service
  (`AA_DB_HOST=mariadb`). Valkey: `valkey.database.svc.cluster.local`, db 0 is the celery broker and db 1 the cache.
- **Restore a dump:** `mariadb-dump --single-transaction --routines --events --triggers --databases sspti_aa` plus `SHOW CREATE USER` / `SHOW GRANTS` of
  `sspti_aa`@`%`; see `overlays/magi/database/mariadb/README.md` for the client.
