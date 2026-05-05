from .base import *

SECRET_KEY = os.environ.get("AA_SECRET_KEY")
SITE_NAME = os.environ.get("AA_SITENAME")

def _env_bool(name: str, default: bool = False) -> bool:
    value = os.environ.get(name)
    if value is None:
        return default
    return str(value).strip().lower() in {"1", "true", "yes", "y", "on"}

protocol = (os.environ.get("PROTOCOL") or "https").strip()
if "://" not in protocol:
    protocol = protocol.rstrip("/") + "://"
if not protocol.endswith("://"):
    protocol = protocol.split("://", 1)[0] + "://"

auth_subdomain = (os.environ.get("AUTH_SUBDOMAIN") or "").strip().strip(".")
domain = (os.environ.get("DOMAIN") or "").strip()
site_host = f"{auth_subdomain}.{domain}" if auth_subdomain else domain

SITE_URL = (os.environ.get("AA_SITE_URL") or "").strip() or f"{protocol}{site_host}"

trusted_origins = set()
if SITE_URL:
    trusted_origins.add(SITE_URL.rstrip("/"))
if domain:
    trusted_origins.add(f"{protocol}{domain}".rstrip("/"))
    trusted_origins.add(f"{protocol}*.{domain}".rstrip("/"))

extra_trusted = (os.environ.get("AA_CSRF_TRUSTED_ORIGINS") or "").strip()
if extra_trusted:
    for origin in extra_trusted.replace("\n", ",").split(","):
        origin = origin.strip()
        if origin:
            trusted_origins.add(origin.rstrip("/"))

CSRF_TRUSTED_ORIGINS = sorted(trusted_origins)
DEBUG = _env_bool("AA_DEBUG", False)
DATABASES["default"] = {
    "ENGINE": "django.db.backends.mysql",
    "NAME": os.environ.get("AA_DB_NAME"),
    "USER": os.environ.get("AA_DB_USER"),
    "PASSWORD": os.environ.get("AA_DB_PASSWORD"),
    "HOST": os.environ.get("AA_DB_HOST"),
    "PORT": os.environ.get("AA_DB_PORT", "3306"),
    "OPTIONS": {"charset": os.environ.get("AA_DB_CHARSET", "utf8mb4")},
}
ESI_SSO_CALLBACK_URL = f"{SITE_URL}/sso/callback"  # Do NOT change this line!
ESI_SSO_CLIENT_ID = os.environ.get("ESI_SSO_CLIENT_ID")
ESI_SSO_CLIENT_SECRET = os.environ.get("ESI_SSO_CLIENT_SECRET")
ESI_USER_CONTACT_EMAIL = os.environ.get("ESI_USER_CONTACT_EMAIL")
REGISTRATION_VERIFY_EMAIL = False
EMAIL_HOST = os.environ.get("AA_EMAIL_HOST", "")
EMAIL_PORT = os.environ.get("AA_EMAIL_PORT", 587)
EMAIL_HOST_USER = os.environ.get("AA_EMAIL_HOST_USER", "")
EMAIL_HOST_PASSWORD = os.environ.get("AA_EMAIL_HOST_PASSWORD", "")
EMAIL_USE_TLS = os.environ.get("AA_EMAIL_USE_TLS", True)
DEFAULT_FROM_EMAIL = os.environ.get("AA_DEFAULT_FROM_EMAIL", "")

ROOT_URLCONF = "myauth.urls"
WSGI_APPLICATION = "myauth.wsgi.application"
STATIC_ROOT = "/var/www/myauth/static/"
BROKER_URL = f"redis://{os.environ.get('AA_REDIS', 'redis:6379')}/0"
CACHES = {
    "default": {
        "BACKEND": "django_redis.cache.RedisCache",
        "LOCATION": f"redis://{os.environ.get('AA_REDIS', 'redis:6379')}/1",
    }
}

AA_EXTRA_INSTALLED_APPS = os.environ.get("AA_EXTRA_INSTALLED_APPS", "").strip()
if AA_EXTRA_INSTALLED_APPS:
    extra_apps = [
        app.strip()
        for app in AA_EXTRA_INSTALLED_APPS.replace("\n", ",").split(",")
        if app.strip()
    ]
    INSTALLED_APPS += extra_apps

DISCORD_CALLBACK_URL = f"{SITE_URL}/discord/callback/"  # Do NOT change this line!
DISCORD_GUILD_ID = os.environ.get("DISCORD_GUILD_ID", "")
DISCORD_APP_ID = os.environ.get("DISCORD_APP_ID", "")
DISCORD_APP_SECRET = os.environ.get("DISCORD_APP_SECRET", "")
DISCORD_BOT_TOKEN = os.environ.get("DISCORD_BOT_TOKEN", "")
DISCORD_SYNC_NAMES = _env_bool("DISCORD_SYNC_NAMES", False)