from django.urls import include, path

from allianceauth import urls
from health_check.views import HealthCheckView


class OverallHealthCheckView(HealthCheckView):
    checks = (
        "health_check.checks.Cache",
        "health_check.checks.Database",
        # The default hostname is the pod name, which cluster DNS does not publish; resolve the Service instead.
        ("health_check.checks.DNS", {"hostname": "aa-sspti-web.sspti.svc.cluster.local"}),
        "health_check.checks.Storage",
    )


class DatabaseHealthCheckView(HealthCheckView):
    checks = ("health_check.checks.Database",)


class CacheHealthCheckView(HealthCheckView):
    checks = ("health_check.checks.Cache",)


class StorageHealthCheckView(HealthCheckView):
    checks = ("health_check.checks.Storage",)


class CeleryHealthCheckView(HealthCheckView):
    checks = ("health_check.contrib.celery.Ping",)


urlpatterns = [
    path('health/', OverallHealthCheckView.as_view(), name='health_check'),
    path('health/db/', DatabaseHealthCheckView.as_view(), name='health_check_db'),
    path('health/cache/', CacheHealthCheckView.as_view(), name='health_check_cache'),
    path('health/storage/', StorageHealthCheckView.as_view(), name='health_check_storage'),
    path('health/celery/', CeleryHealthCheckView.as_view(), name='health_check_celery'),
    path('', include(urls)),
]

handler500 = 'allianceauth.views.Generic500Redirect'
handler404 = 'allianceauth.views.Generic404Redirect'
handler403 = 'allianceauth.views.Generic403Redirect'
handler400 = 'allianceauth.views.Generic400Redirect'
