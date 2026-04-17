-- Initialize VectorChord and pgvector extensions
-- This script runs automatically when the PostgreSQL container starts

-- Create extensions if they don't exist
CREATE EXTENSION IF NOT EXISTS pgvector;
CREATE EXTENSION IF NOT EXISTS vectorchord;

-- Create a sample table to demonstrate VectorChord usage
CREATE TABLE IF NOT EXISTS embeddings (
    id BIGSERIAL PRIMARY KEY,
    text TEXT NOT NULL,
    embedding vector(1536),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create index for vector search optimization (optional)
-- This can improve query performance for large datasets
-- CREATE INDEX ON embeddings USING hnsw (embedding vector_cosine_ops);

-- Verify extensions are loaded
SELECT extname, extversion FROM pg_extension 
WHERE extname IN ('pgvector', 'vectorchord')
ORDER BY extname;
