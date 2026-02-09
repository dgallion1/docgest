# Document Ingestion Service (docgest)

Standalone Go service that ingests documents and writes extracted facts into pathstore.

## Quick Start

```bash
# Required env vars
export PATHSTORE_API_KEY=dev-key-please-change
export DOCGEST_API_KEY=my-docgest-key
export ANTHROPIC_API_KEY=sk-ant-...
export ANTHROPIC_MODEL=claude-sonnet-4-5-20250929

# Run (requires pathstore on :8080)
go run ./cmd/server

# Or with Docker Compose (includes pathstore + postgres)
docker compose up
```

## Development

```bash
go build ./...          # Build
go vet ./...            # Lint
go test ./...           # Unit tests (44 tests)
```

## API

All endpoints except `/health` require `Authorization: Bearer <DOCGEST_API_KEY>`.

```bash
# Ingest a document
curl -X POST http://localhost:8090/api/ingest \
  -H "Authorization: Bearer $DOCGEST_API_KEY" \
  -F file=@document.md \
  -F user_id=test-user

# Check job status
curl http://localhost:8090/api/ingest/{job_id}/status \
  -H "Authorization: Bearer $DOCGEST_API_KEY"

# List user's documents
curl "http://localhost:8090/api/documents?user_id=test-user" \
  -H "Authorization: Bearer $DOCGEST_API_KEY"

# Delete document and its facts
curl -X DELETE "http://localhost:8090/api/documents/{doc_id}?user_id=test-user" \
  -H "Authorization: Bearer $DOCGEST_API_KEY"
```

## Project Layout

```
cmd/server/          Main entrypoint
internal/api/        HTTP handlers, auth middleware (chi router)
internal/config/     Centralized config from env vars
internal/doctree/    DocTree/DocNode/Chunk types
internal/parser/     Format parsers (TXT, Markdown, CSV, HTML, PDF, DOCX)
internal/chunker/    Structure-aware recursive text splitter
internal/extract/    Claude API client, extraction prompt, fact validation
internal/pipeline/   Orchestrator, worker pool, job state machine, retry
internal/pathstore/  HTTP client for pathstore API
```

## Supported Formats

TXT, Markdown, CSV, HTML, PDF (with pdftotext fallback), DOCX

## Pipeline

`Upload → Parse → DocTree → Chunk (structure-aware) → Extract (Claude) → Validate → Store Facts → Write Manifest`

Facts are stored in the same paths as chat-agent extraction. A manifest under
`documents/{doc_id}/facts/` enables exact deletion without scanning entity/topic trees.
