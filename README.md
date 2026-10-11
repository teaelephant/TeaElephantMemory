# TeaElephantMemory

![TeaElephant Memory architecture — iOS and macOS clients, Go GraphQL API, PostgreSQL and AI integration](docs/images/teaelephant-backend-social-preview.jpg)

Go backend for TeaElephant's iOS application and macOS catalogue editor.

## Features

- GraphQL HTTP and WebSocket API for teas, tags, QR records, and user collections
- PostgreSQL persistence and consumption history
- AI-generated descriptions and tea recommendations, including Tea of the Day
- Apple Sign In, admin JWT authentication, and APNs expiration notifications
- OpenWeather context and printable QR code generation

## Requirements

- Go 1.27.2 or newer, as specified in `go.mod`
- PostgreSQL; the local Compose setup uses PostgreSQL 18.6
- Apple authentication/APNs credentials and an admin verification public key
- OpenAI and OpenWeather API credentials for the corresponding features
- Docker Compose for the container-based setup

## Local setup with Docker Compose

1. Copy the configuration template:

   ```sh
   cp .env.example .env
   ```

2. Fill in the API credentials and Apple identifiers in `.env`. Set
   `APPLE_AUTH_SECRET_FILE` to your existing Apple private key and
   `ADMIN_PUBLIC_KEY_FILE` to the admin public key. The default paths are
   `secrets/AuthKey.p8` and `secrets/admin_public_key.pem`.
   See [admin key setup](docs/ADMIN_KEYS_SETUP.md) for generating the admin keypair.
3. Build and start the services:

   ```sh
   docker compose up -d --build
   ```

Compose builds the current server sources, waits for PostgreSQL to become
healthy, and mounts the two key files read-only. The default endpoints are:

- GraphQL HTTP: `http://localhost:8080/v2/query`
- GraphQL WebSocket: `ws://localhost:8080/v2/query`
- GraphQL playground: `http://localhost:8080/v2/`
- HTTP health: `http://localhost:8080/health`
- Prometheus metrics: `http://localhost:8080/metrics`
- PostgreSQL: `localhost:5432`

The PostgreSQL service stores data in the named `postgres-data-v18` volume,
mounted at `/var/lib/postgresql`. PostgreSQL 18 uses the version-specific
data directory `/var/lib/postgresql/18/docker`.
The [official image](https://hub.docker.com/_/postgres) applies `db/schema.sql`
during initialization of an empty data directory. Existing databases need
schema changes applied explicitly.

`docker compose down` stops the services while retaining the database volume.

### Existing PostgreSQL 17 data

The former `postgres-data` volume is preserved and is not mounted into
PostgreSQL 18. A new volume prevents starting the new major against an
incompatible old data directory. If you already have data, back it up using
PostgreSQL 17 and perform a reviewed dump/restore or
[`pg_upgrade`](https://www.postgresql.org/docs/18/upgrading.html) before
switching to the new database. Starting Compose alone initializes an empty
PostgreSQL 18 database; it does not migrate your existing data.

## Run the server from source

Start only the local database:

```sh
docker compose up -d postgres
```

After configuring `.env`, load it into the shell and set the runtime key paths:

```sh
set -a
. ./.env
set +a
export APPLE_AUTH_SECRET_PATH="$APPLE_AUTH_SECRET_FILE"
export ADMIN_PUBLIC_KEY_PATH="$ADMIN_PUBLIC_KEY_FILE"
export PG_DSN="postgres://teaelephant:teaelephant@localhost:5432/teaelephant?sslmode=disable"
go run ./cmd/server
```

Adjust `PG_DSN` if you change the local database credentials or port.

## Runtime configuration

| Variable | Purpose |
|---|---|
| `PG_DSN` | Required PostgreSQL connection string; startup checks connectivity |
| `LOG_LEVEL` | Logging level, default `info` |
| `OPEN_AI_TOKEN` | OpenAI API token for descriptions and recommendations |
| `OPENWEATHER_APIKEY` | OpenWeather API key |
| `APPLE_AUTH_CLIENT_ID` | Apple client ID and APNs topic |
| `APPLE_AUTH_TEAM_ID` | Apple developer team ID |
| `APPLE_AUTH_KEY_ID` | Apple signing key ID |
| `APPLE_AUTH_SECRET_PATH` | Apple private key file used by auth and APNs |
| `ADMIN_PUBLIC_KEY_PATH` | Admin public key file; default `/keys/admin/admin_public_key.pem` |

The server listens on port 8080. `SERVER_PORT` and `POSTGRES_PORT` in
`.env.example` configure Compose host ports; `APPLE_AUTH_SECRET_FILE` and
`ADMIN_PUBLIC_KEY_FILE` configure its host file mounts.

### OpenAI models

| Workload | Model | Reasoning effort |
|---|---|---|
| Descriptions, recommendations, and Tea of the Day context scores | `gpt-6-astra` | API default |
| Streaming recommendations | `gpt-6-luna` | `none` |

These defaults follow the current [GPT-6 model guidance](https://developers.openai.com/api/docs/guides/latest-model).
Both models support the existing text-only Chat Completions integration.
Streaming retains a separate fast model and sends `reasoning_effort: "none"`.
The local tests validate HTTP request parameters, completion parsing, JSON
scores, and SSE delivery against a test server. They do not contact OpenAI;
model access and response quality with the deployment's API key need live
verification separately.

## Build and checks

Install the pinned linter and SQL generator into the ignored `bin` directory:

```sh
make tools
```

`Makefile` pins golangci-lint and sqlc. The Go module's `tool` directives pin
gqlgen and mockgen alongside their dependencies. The generators run through
`go tool`, using the module's versions rather than globally installed binaries.

```sh
go build ./...
go test ./...
go vet ./...
./bin/golangci-lint run
```

The OpenWeather integration test runs only when `KEY` is set. Other current
tests use local data and do not need application credentials.

Regenerate all configured GraphQL, mock, and SQL sources:

```sh
make generate
```

For GraphQL alone, run `(cd pkg/api/v2 && go tool gqlgen generate)`.
See the SQL generation-path note in [PostgreSQL storage](docs/POSTGRES_STORAGE.md).

Generate printable QR codes with `go run ./cmd/qr_gen`.

## Project layout

- `cmd/server`: application entrypoint and dependency wiring
- `cmd/qr_gen`, `printqr`: printable QR generation
- `pkg/api/v2`: GraphQL schema, resolvers, and models
- `pkg/pg`, `pkg/pgstore`: runtime PostgreSQL adapter and query helpers
- `db/schema.sql`, `db/queries`: relational schema and SQL definitions
- `internal`: managers, auth, recommendations, weather, consumption, and APNs
- `deployment/server.yml`: Kubernetes server deployment
- `static`: landing page and Apple app-site association

See [PostgreSQL storage](docs/POSTGRES_STORAGE.md) for data relationships and
schema maintenance. License: [LICENSE](LICENSE).
