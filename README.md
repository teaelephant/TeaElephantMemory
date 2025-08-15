# TeaElephantMemory

A Go service powering Tea Elephant Memory: store tea collections, tags, and notifications, generate printable QR codes, and expose a GraphQL API. Includes integrations for Apple Push Notifications and OpenWeather.

## Features
- GraphQL API (gqlgen) for collections, teas, tags, and notifications
- FoundationDB-backed storage (via lightweight client in pkg/fdbclient)
- QR code generation utilities (printqr)
- APNs notification sender
- OpenWeather integration for weather-aware features
- Docker and docker-compose support

## Prerequisites
- Go 1.22+
- Docker and Docker Compose (optional but recommended)
- FoundationDB (local dev cluster) or access to a running FDB

## Quick Start (Docker Compose)
1. Copy or verify FDB cluster config in `config/fdb.cluster`.
2. Start services:
   - docker compose up -d
3. Server listens on default port (see flags/env below). GraphQL schema is under `pkg/api/v2/graphql`.

To stop:
- docker compose down

## Build From Source
1. Ensure Go toolchain is installed (1.22+).
2. Download modules and build:
   - go mod download
   - go build ./...

### Run the server
- go run ./cmd/server

Common flags/env (examples):
- FDB_CLUSTER_FILE: path to FoundationDB cluster file (default: config/fdb.cluster)
- PORT: server port (default may be 8080)

### Generate QR codes (CLI)
- go run ./cmd/qr_gen
Outputs printable QR codes using logic from `printqr` package.

## Tests
- Run unit tests:
  - go test ./...

Some packages have dedicated tests, e.g.:
- internal/openweather
- internal/adviser
- common packages

## Linting
A local golangci-lint binary exists under `bin/golangci-lint`. If desired:
- bin/golangci-lint run

## Project Layout Highlights
- cmd/server: HTTP server entrypoint
- cmd/qr_gen: CLI QR generator
- pkg/api/v2/graphql: GraphQL schema and generated code
- pkg/fdb, pkg/fdbclient: FoundationDB access layers
- internal/*: domain services and managers (adviser, descrgen, openweather, managers for tags/tea/collection)
- printqr: QR image creation
- static/: static assets, including apple-app-site-association

## Configuration Notes
- FoundationDB cluster file: `config/fdb.cluster` (ensure it matches your environment)
- APNs requires credentials (e.g., AuthKey_*.p8). Place and configure securely for production.
- Environment variables and flags may be introduced/used by individual components; check respective packages for details.

## License
This project is licensed under the MIT License. See the LICENSE file for details.