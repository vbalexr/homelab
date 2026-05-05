FROM python:3.12-slim
ARG AUTH_VERSION=v5.0.0a4
ARG AUTH_PACKAGE=allianceauth==${AUTH_VERSION}
ENV AUTH_USER=allianceauth
ENV AUTH_GROUP=allianceauth
ENV AUTH_USERGROUP=${AUTH_USER}:${AUTH_GROUP}
ENV STATIC_BASE=/var/www
ENV AUTH_HOME=/home/allianceauth

# Setup user and directory permissions
SHELL ["/bin/bash", "-c"]
RUN groupadd -g 61000 ${AUTH_GROUP} && \
    useradd -g 61000 -l -m -s /bin/false -u 61000 ${AUTH_USER}

RUN mkdir -p ${STATIC_BASE}/myauth/static \
    && chown ${AUTH_USERGROUP} ${STATIC_BASE}/myauth/static

# Install Build Dependencies
RUN apt-get update \
    && apt-get upgrade -y  \
    && apt-get install -y --no-install-recommends libmariadb-dev gcc git pkg-config \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

# Install AA and Dependencies
RUN pip install --no-cache-dir ${AUTH_PACKAGE} gunicorn

# Switch to non-root user
USER ${AUTH_USER}
WORKDIR ${AUTH_HOME}

# Initialize auth
RUN allianceauth start myauth
COPY /allianceauth/project_template/project_name/settings/local.py ${AUTH_HOME}/myauth/myauth/settings/local.py
RUN allianceauth update myauth

RUN echo 'alias auth="python $AUTH_HOME/myauth/manage.py"' >> ~/.bashrc && \
    source ~/.bashrc

ENTRYPOINT ["sh", "-c"]
