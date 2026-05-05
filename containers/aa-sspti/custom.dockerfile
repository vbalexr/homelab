ARG AA_DOCKER_TAG=registry.gitlab.com/allianceauth/allianceauth/auth:v4.13.1
FROM $AA_DOCKER_TAG

WORKDIR ${AUTH_HOME}

COPY /conf/requirements.txt requirements.txt
RUN pip install -r requirements.txt

COPY /conf/local.py /home/allianceauth/myauth/myauth/settings/local.py
COPY /conf/celery.py /home/allianceauth/myauth/myauth/celery.py
COPY /conf/urls.py /home/allianceauth/myauth/myauth/urls.py
COPY --chmod=0755 /conf/memory_check.sh /memory_check.sh
