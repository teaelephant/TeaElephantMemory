TeaElephantMemory — Advanced Development Guidelines

This document captures project-specific knowledge to speed up development and reduce setup friction. It assumes familiarity with Go, Docker, FoundationDB, GraphQL (gqlgen), and general backend practices.

1. Build and Configuration

1.1 Languages and Toolchain
- Go: 1.25 (module sets go 1.25.0 in go.mod). Ensure your local toolchain matches to avoid stdlib/API drift.
- FoundationDB client libraries are required to run the server locally (runtime, not compile time). The Dockerfile installs foundationdb-clients; for local runs, install Apple FoundationDB clients for your OS.

1.2 Server binary
- Build: go build -v -o ./bin/server ./cmd/server
  - The build includes CGO and links FoundationDB at runtime; you still need FDB clients at execution time.
- Run (local):
  - Prerequisites: FoundationDB service running and a cluster file accessible.
  - Minimal env configuration:
    - LOG_LEVEL: one of panic,fatal,error,warn,info,debug,trace (default info)
    - DATABASEPATH: path to fdb.cluster (default /usr/local/etc/foundationdb/fdb.cluster)
    - OPEN_AI_TOKEN: required at runtime (used by description generator and adviser)
    - APPLE_AUTH_CLIENT_ID, APPLE_AUTH_TEAM_ID, APPLE_AUTH_KEY_ID: required at runtime
    - APPLE_AUTH_SECRET_PATH: defaults to AuthKey_39D5B439QV.p8 in project root; should point to your .p8 key
  - Example run:
    DATABASEPATH=/usr/local/etc/foundationdb/fdb.cluster \
    LOG_LEVEL=debug \
    OPEN_AI_TOKEN=sk-... \
    APPLE_AUTH_CLIENT_ID=xax.TeaElephant \
    APPLE_AUTH_TEAM_ID=YOUR_TEAM \
    APPLE_AUTH_KEY_ID=KEYID123 \
    APPLE_AUTH_SECRET_PATH=/absolute/path/to/AuthKey_xxx.p8 \
    ./bin/server

1.3 Docker
- Build image:
  docker build --build-arg VERSION=$(git rev-parse --short HEAD) -t teaelephant/server:dev .
- Run with FoundationDB:
  - You can mount a cluster file to /usr/local/etc/foundationdb/fdb.cluster inside the container or pass DATABASEPATH to point to the mounted location.
- Provided docker-compose.yaml shows an example stack with a FoundationDB container. Adjust volumes/paths for your system.

1.4 Kubernetes deployment
- deployment/server.yml shows expected environment variables and volumes:
  - DATABASEPATH is set to /etc/fdb/fdb.cluster via a ConfigMap.
  - APPLE_AUTH_* come from a Secret; OPEN_AI_TOKEN and OPENWEATHER_APIKEY also from Secrets.
- Probes expose /health on port 8080; metrics are scraped from /metrics (Prometheus annotations present).

1.5 GraphQL codegen (gqlgen)
- Config file: pkg/api/v2/gqlgen.yml
- Generate after schema/model changes:
  (cd pkg/api/v2 && go run github.com/99designs/gqlgen generate)
- The schema lives under pkg/api/v2/graphql/*.graphql. Resolvers are under pkg/api/v2/graphql; models under pkg/api/v2/models.

2. Testing

2.1 Running tests
- Fast path: go test ./...
  - External service tests are safe by default:
    - internal/openweather/service_test.go requires env KEY; if absent, it calls t.Skip(...). No network calls occur otherwise.
    - internal/adviser/service_test.go uses template rendering only (no OpenAI calls).
- Run a single package or test for fast iteration:
  go test ./internal/consumption -run TestMemoryStore -v

2.2 Adding new tests
- Prefer self-contained unit tests that do not require external services. Use interfaces and small adapters for boundary code.
- If a test requires external credentials, gate it behind an env var and skip when not provided (pattern used in openweather tests).
- Use testify for assertions/requirements (github.com/stretchr/testify is already a dependency).

2.3 Demonstration: create and run a simple test (validated)
The following example illustrates how to add a self-contained test for the in-memory consumption store. This exact snippet was validated locally before documenting.

File: internal/consumption/store_test.go

package consumption

import (
    "context"
    "testing"
    "time"

    "github.com/google/uuid"
    "github.com/stretchr/testify/require"
)

func TestMemoryStore_RecordAndRecent_WithRetentionAndSorting(t *testing.T) {
    ctx := context.Background()
    userID := uuid.New()
    teaA := uuid.New()
    teaB := uuid.New()

    // Set small retention to test trimming
    store := NewMemoryStore(2 * time.Hour)

    base := time.Now().Add(-3 * time.Hour)

    // Outside retention window (should be trimmed after later record)
    require.NoError(t, store.Record(ctx, userID, teaA, base))
    // Inside retention window
    require.NoError(t, store.Record(ctx, userID, teaB, base.Add(2*time.Hour)))
    require.NoError(t, store.Record(ctx, userID, teaA, base.Add(2*time.Hour+30*time.Minute)))

    // Query since very old time to include everything retained
    items, err := store.Recent(ctx, userID, base.Add(-24*time.Hour))
    require.NoError(t, err)

    // The first event is older than retention and should be trimmed
    require.Len(t, items, 2)

    // Ensure sorted by time desc
    require.True(t, items[0].Time.After(items[1].Time) || items[0].Time.Equal(items[1].Time))
}

Run only this package’s tests:
- go test ./internal/consumption -run TestMemoryStore_RecordAndRecent_WithRetentionAndSorting -v

Note: After verifying, you can remove the temporary test if it was created only as a demo. In normal development, keep tests under version control.

3. Additional Development Notes

3.1 Runtime services and secrets
- FoundationDB: The app uses FoundationDB for persistence. Ensure foundationdb-clients are installed locally and the database is reachable. Use DATABASEPATH to point to your fdb.cluster file. A sample cluster file exists under config/fdb.cluster; adapt pathing as needed.
- OpenAI: OPEN_AI_TOKEN is required for runtime (adviser, description generator). For development without external calls, avoid invoking endpoints that require these components, or inject test doubles.
- Apple Auth/APNs: APPLE_AUTH_* vars and .p8 key file are required to start the server. For local development where APNs is not needed, you can isolate and test subpackages instead of running the server, or provide placeholder values and a dummy .p8 file only if you know what you’re doing. The server will panic at startup if these are misconfigured.

3.2 Code style and linting
- Idiomatic Go formatting via gofmt/goimports.
- Static analysis:
  - go vet ./...
  - Qodana (qodana.yaml present) can be used in CI; try locally with Docker: docker run --rm -it -v $(pwd):/data/project -p 8080:8080 jetbrains/qodana-go:latest
  - After each change, run: ./bin/golangci-lint run --new-from-rev=origin/master and fix all reported issues.
- Test helpers: testify is available (assert/require). Prefer require for setup expectations to fail fast in tests.

3.3 GraphQL and resolvers
- Schema-driven development with gqlgen. After updating schema.graphql, re-run codegen and implement any new resolver stubs in pkg/api/v2/graphql.
- Custom scalars/mappings are defined in pkg/api/v2/gqlgen.yml (e.g., ID mapped to pkg/api/v2/common.ID, Date to graphql.Time). Keep mappings in sync when introducing new types.

3.4 Observability and ops
- Metrics: /metrics endpoint is enabled for Prometheus (see deployment annotations). Add new metrics via prometheus/client_golang if extending subsystems.
- Health: /health endpoint is used by k8s probes; keep it lightweight and dependency-aware.

3.5 Local iteration tips
- Focus development and tests on isolated packages (e.g., internal/consumption, internal/adviser) to avoid external dependencies.
- For features requiring FDB, prefer introducing interfaces to allow in-memory fakes during unit testing (MemoryStore is a good example).
- When adding network-dependent tests, guard them behind environment flags and Skip when not configured.

Appendix: Verified Commands (2025-08-21 10:25)
- go test ./internal/consumption -run TestMemoryStore_RecordAndRecent_WithRetentionAndSorting -v  (PASS)
- The test file used is included above as a snippet and was removed after verification, as this document suggests keeping the repository clean for this task.
