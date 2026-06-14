# Stan Backend Tech Lead Challenge

This is a standalone Go service for the streaming availability challenge.

## Run locally

```bash
make run
```

Environment variables:

- `PORT` sets the listen port for deployment environments such as Render.
- `ADDR` overrides the full listen address when `PORT` is not set.

## Test

```bash
go test ./...
go test -race ./...
```

## Design

The repository follows the common Go project layout:

- `cmd/nine` contains the executable entrypoint.
- `internal/app` contains the application bootstrap only.
- `internal/app/httpapi` contains the HTTP transport and integration tests.
- `internal/app/store` contains the in-memory registry and per-asset state.
- `internal/app/model` contains request, response, and domain types.
- The root module stays small and only holds repo-level documentation and metadata.

The store uses a `sync.Map` for the asset registry, and each asset has its own lock and segment map. That avoids a single mutex around the whole in-memory store while keeping the concurrency model simple. Ingest is an upsert keyed by `(asset_id, segment_index)`, so duplicate deliveries overwrite the same slot instead of creating duplicates. Manifest reads take a per-asset snapshot, copy the segments, and sort them before returning JSON.

Assumptions:
- State is ephemeral and is reset on restart.
- `received_at` is treated as required input and is used to advance `last_updated`.
- Manifest ordering is by `segment_index` only.
