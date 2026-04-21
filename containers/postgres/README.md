# PostgreSQL 16 with VectorChord

Production Docker image: PostgreSQL 16 + pgvector + VectorChord.

## Features

- **Base**: PostgreSQL 16 on Debian
- **Extensions**: pgvector, VectorChord pre-installed
- **Multi-arch**: linux/amd64, linux/arm64
- **Health checks**: Built-in readiness probes

## Build

```bash
docker build -t ghcr.io/vbalexr/postgres:latest .
```

## Run

```bash
docker run -e POSTGRES_PASSWORD=secret -p 5432:5432 ghcr.io/vbalexr/postgres:latest
```

## Extensions

### pgvector
```sql
CREATE EXTENSION IF NOT EXISTS vector;
```

### VectorChord
```sql
CREATE EXTENSION IF NOT EXISTS vectorchord;
```
    embedding vector(1536)  -- 1536-dimensional vectors (e.g., from OpenAI)
);

-- Insert sample data
INSERT INTO documents (content, embedding) VALUES
    ('Hello world', '[0.1, 0.2, ..., 0.5]'::vector),
    ('PostgreSQL rocks', '[0.2, 0.3, ..., 0.6]'::vector);

-- Search for similar vectors (cosine distance)
SELECT id, content, embedding <-> '[0.15, 0.25, ..., 0.55]'::vector AS distance
FROM documents
ORDER BY distance
LIMIT 5;
```

#### Supported Distance Operators
- `<->` - Cosine distance
- `<#>` - Negative inner product
- `<=>` - Euclidean distance

#### Create Vector Indexes
```sql
-- IVFFlat index (good for large datasets)
CREATE INDEX ON documents USING ivfflat (embedding vector_cosine_ops)
    WITH (lists = 100);

-- HNSW index (better for dynamic workloads)
CREATE INDEX ON documents USING hnsw (embedding vector_cosine_ops)
    WITH (m = 16, ef_construction = 200);
```

### VectorChord

VectorChord is an advanced PostgreSQL extension providing high-performance vector indexing and search capabilities.

#### Enable VectorChord
```sql
-- Create the extension
CREATE EXTENSION IF NOT EXISTS vectorchord;

-- Verify installation
SELECT extversion FROM pg_extension WHERE extname = 'vectorchord';
```

#### Create Vector Indexes
```sql
-- Create a table with vector column
CREATE TABLE embeddings (
    id SERIAL PRIMARY KEY,
    data JSONB,
    vector REAL[] NOT NULL
);

-- Create an IVFFlat index using VectorChord
CREATE INDEX ON embeddings USING ivfflat (vector vector_l2_ops)
    WITH (lists = 100);

-- Alternative: HNSW index
CREATE INDEX ON embeddings USING hnsw (vector vector_l2_ops)
    WITH (m = 16, ef_construction = 200);
```

#### Query with Vector Indexes
```sql
-- Insert sample vectors
INSERT INTO embeddings (data, vector) VALUES
    ('{"name": "doc1"}', ARRAY[0.1, 0.2, 0.3]::REAL[]),
    ('{"name": "doc2"}', ARRAY[0.15, 0.25, 0.35]::REAL[]);

-- Search using L2 distance (will use index if available)
SELECT id, data, vector <-> ARRAY[0.12, 0.22, 0.32]::REAL[] AS distance
FROM embeddings
ORDER BY distance
LIMIT 10;

-- Search using cosine similarity
SELECT id, data
FROM embeddings
ORDER BY vector <=> ARRAY[0.12, 0.22, 0.32]::REAL[]
LIMIT 10;
```

#### Index Types Available
- **IVFFlat** - Good for large static datasets, memory efficient
- **HNSW** - Hierarchical Navigable Small World, better for dynamic data
- **DiskANN** - For very large datasets that don't fit in memory

### Using Both Extensions Together

For optimal performance with both pgvector and VectorChord:

```sql
-- Enable both
CREATE EXTENSION IF NOT EXISTS vector;
CREATE EXTENSION IF NOT EXISTS vectorchord;

-- Create table using pgvector types
CREATE TABLE ai_embeddings (
    id BIGSERIAL PRIMARY KEY,
    document_id TEXT NOT NULL,
    text_content TEXT,
    embedding vector(1536),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create HNSW index for fast similarity search
CREATE INDEX ON ai_embeddings USING hnsw (embedding vector_cosine_ops)
    WITH (m = 16, ef_construction = 200);

-- Verify indexes
SELECT schemaname, tablename, indexname, indexdef 
FROM pg_indexes 
WHERE tablename = 'ai_embeddings';

-- Example: Semantic search
SELECT 
    id,
    document_id,
    text_content,
    1 - (embedding <=> '[0.1, 0.2, ..., 0.5]'::vector) AS similarity_score
FROM ai_embeddings
WHERE 1 - (embedding <=> '[0.1, 0.2, ..., 0.5]'::vector) > 0.7
ORDER BY similarity_score DESC
LIMIT 10;
```

### Performance Tuning

When using large vector datasets:

```sql
-- Set appropriate work_mem for vector operations
SET work_mem = '256MB';

-- Enable parallel query execution
SET max_parallel_workers_per_gather = 4;

-- For IVFFlat indexes, tune list count
-- lists = sqrt(total_rows) is a good starting point
-- For 1M rows: lists = 1000

-- Monitor index usage
SELECT * FROM pg_stat_user_indexes 
WHERE relname = 'ai_embeddings';
```

### Automatic Initialization

Both extensions are automatically pre-loaded when the container starts:
```dockerfile
ENV POSTGRES_INITDB_ARGS="-c shared_preload_libraries=vectorchord,pgvector"
```

This means they are available immediately in any database without manual loading.

## Build Automation

### GitHub Actions Workflows

#### `build-postgres-image.yml` - Container Build Pipeline
Automatically builds and pushes the image to `ghcr.io/vbalexr/postgres` when:
- Changes are pushed to `containers/postgres/` folder
- Git tags matching `v*` are created
- Manual trigger via GitHub UI

**Features**:
- Multi-architecture builds (amd64, arm64)
- Semantic versioning with git tags
- Build caching for faster iterations
- Pushes to GitHub Container Registry

#### `check-postgres-updates.yml`
Automatically checks for upstream updates every Monday at 02:00 UTC:
- Queries pgvector GitHub tags (latest release)
- Queries VectorChord GitHub releases
- Updates `version.txt` if newer versions available
- Commits changes and triggers rebuild

## Version Tracking

**File**: `version.txt`

Used by build workflows to determine current versions and trigger updates:
```
PG_MAJOR_VERSION=18
PGVECTOR_VERSION=0.8.2
VECTORCHORD_VERSION=1.1.1
BUILD_DATE=2026-04-20
```

To update versions manually:
```bash
# Edit the file
vi version.txt

# Commit and push
git add version.txt
git commit -m "chore: update dependency versions"
git push
```

## Customization

To customize the container:

1. **Change versions** - Edit `version.txt` and rebuild
2. **Modify Dockerfile** - Add packages, change base image, etc.
3. **Add scripts** - Place custom SQL or shell scripts and copy them in

Example custom Dockerfile:
```dockerfile
FROM ghcr.io/vbalexr/postgres:latest

# Add custom tools
RUN apt-get update && apt-get install -y pgbackrest && rm -rf /var/lib/apt/lists/*

# Copy custom initialization script
COPY ./init-custom.sql /docker-entrypoint-initdb.d/
```

## Troubleshooting

### Container fails to start
```bash
docker logs postgres-test
```

### Extensions not loaded
```bash
docker exec postgres-test psql -U postgres -c "SELECT extname FROM pg_extension;"
```

### Verify versions in container
```bash
docker exec postgres-test psql -U postgres -c "SELECT version();"
```

### Check vector extension functions
```bash
docker exec postgres-test psql -U postgres -c "\df *vector*"
```

## Related Resources

- [PostgreSQL Official Docs](https://www.postgresql.org/docs/16/)
- [pgvector GitHub](https://github.com/pgvector/pgvector)
- [VectorChord GitHub](https://github.com/tensorchord/vectorchord)
- [Docker Documentation](https://docs.docker.com/)
