# TeaElephantMemory Development Guidelines

## Build and configuration

- Use Go 1.25.1 or newer, matching `go.mod`.
- PostgreSQL is the only storage engine. Startup requires `PG_DSN` and verifies
  the database connection before serving requests.
- Storage is implemented by `pkg/pg`, with query helpers in `pkg/pgstore`.
  Consumption history uses the same database through `internal/consumption`.
- Read `README.md`, `.env.example`, and `docs/POSTGRES_STORAGE.md` for local
  setup, schema maintenance, and configuration.

Build all packages with `go build ./...`. Run the server with
`go run ./cmd/server` after configuring the environment and applying the schema.

The Docker Compose setup builds the current server, runs PostgreSQL, initializes
new databases from `db/schema.sql`, and mounts Apple/admin key files read-only.
Application startup also needs the Apple identifiers and key file and an admin
verification public key. API credentials configure OpenAI and OpenWeather.

## Testing

Run `go test ./...` for unit tests. The OpenWeather integration test skips when
`KEY` is unset. The adviser test renders templates without calling OpenAI.

Prefer isolated unit tests and injected adapters for external dependencies.
Gate network-dependent tests behind explicit environment variables.

Useful focused checks:

```sh
go test ./internal/managers/tea/subscribers
go test ./pkg/api/v2/graphql -run TestTeaOfTheDayCache
go test ./internal/auth -run TestRequireUserOrAdmin
```

## Code style and verification

Use idiomatic Go and `gofmt`. Before committing, run:

```sh
go build ./...
go test ./...
go vet ./...
./bin/golangci-lint run --new-from-rev=origin/master
```

Follow the existing linter configuration and avoid unrelated refactoring.

## GraphQL and storage changes

The GraphQL schema is under `pkg/api/v2/graphql`, with resolvers in the same
directory and generated models under `pkg/api/v2/models`.

After GraphQL schema changes, regenerate using:

```sh
(cd pkg/api/v2 && go run github.com/99designs/gqlgen generate)
```

Custom scalars and model mappings are configured in `pkg/api/v2/gqlgen.yml`.
For SQL changes, keep `db/schema.sql`, `db/queries`, and runtime helpers in
`pkg/pgstore` aligned. Check the generation-path note in
`docs/POSTGRES_STORAGE.md` before running sqlc.

## Operations

The HTTP server listens on port 8080. GraphQL uses `/v2/query` over HTTP and
WebSocket; the playground is at `/v2/`. Metrics use `/metrics`, and `/health`
reports that the HTTP process is running.

Kubernetes configuration lives in `deployment/server.yml`. Keep credentials
in environment/secrets and key files; do not commit tokens or private keys.
