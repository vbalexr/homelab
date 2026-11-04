# Build stage for extensions
FROM postgres:18-trixie AS builder

# Build arguments for version tracking
ARG PG_MAJOR_VERSION=18
ARG PGVECTOR_VERSION=0.8.1
ARG VECTORCHORD_VERSION=1.1.1

# Provided automatically by buildx; used to pick portable pgvector build flags
ARG TARGETARCH

# Install build dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    postgresql-server-dev-${PG_MAJOR_VERSION} \
    git \
    wget \
    curl \
    ca-certificates \
    pkg-config \
    clang \
    libclang-dev \
    && rm -rf /var/lib/apt/lists/*

# Install Rust for pgvector and VectorChord compilation
RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y && \
    chmod -R 755 /root/.cargo

# Add Rust to PATH for subsequent RUN commands
ENV PATH=/root/.cargo/bin:$PATH \
    CARGO_HOME=/root/.cargo

# Clone and build pgvector
RUN git clone https://github.com/pgvector/pgvector.git /tmp/pgvector && \
    cd /tmp/pgvector && \
    git checkout v${PGVECTOR_VERSION}

# pgvector's Makefile defaults to OPTFLAGS=-march=native, which bakes the CI
# runner's CPU features (AVX-512) into vector.so and makes backends die with
# SIGILL on nodes that lack them. Pin a portable baseline on amd64 -- x86-64-v2
# is also the minimum Immich v3 requires -- and disable it elsewhere, since the
# Makefile only zeroes OPTFLAGS for Darwin/arm, ppc64 and riscv64, not linux/arm64.
RUN cd /tmp/pgvector && \
    if [ "${TARGETARCH}" = "amd64" ]; then OPTFLAGS="-march=x86-64-v2"; else OPTFLAGS=""; fi && \
    make OPTFLAGS="${OPTFLAGS}" && \
    make install && \
    rm -rf /tmp/pgvector

# Clone and build VectorChord
RUN git clone https://github.com/tensorchord/vectorchord.git /tmp/vectorchord && \
    cd /tmp/vectorchord && \
    git checkout ${VECTORCHORD_VERSION}

RUN cd /tmp/vectorchord && \
    make && \
    make install && \
    rm -rf /tmp/vectorchord

# Runtime stage - use official PostgreSQL image
FROM postgres:18-trixie

LABEL maintainer="homelab"
LABEL description="PostgreSQL 18 with pgvector and VectorChord extensions"

# Copy compiled extensions from builder
COPY --from=builder /usr/lib/postgresql/18/lib/*.so /usr/lib/postgresql/18/lib/
COPY --from=builder /usr/share/postgresql/18/extension/ /usr/share/postgresql/18/extension/

# Install runtime dependencies (minimal set)
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Health check
HEALTHCHECK --interval=10s --timeout=5s --start-period=40s --retries=3 \
    CMD pg_isready -U postgres

EXPOSE 5432

# Default entrypoint from base image (docker-entrypoint.sh)
