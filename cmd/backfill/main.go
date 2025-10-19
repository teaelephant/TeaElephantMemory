// Package main contains the FDB→Postgres backfill utility.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"

	foundationdb "github.com/apple/foundationdb/bindings/go/src/fdb"
	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/teaelephant/TeaElephantMemory/common/key_value/encoder"
	"github.com/teaelephant/TeaElephantMemory/common/key_value/key_builder"
	"github.com/teaelephant/TeaElephantMemory/pkg/fdbclient"
)

const (
	fdbAPIVersion = 710  // FoundationDB API version
	teaBatchSize  = 1000 // batch size for tea inserts
)

const pgSchema = `
-- PostgreSQL schema aligned with docs/FDB_TO_POSTGRES_MIGRATION_PLAN.md
-- This file is used by sqlc for type generation and can also be used as a base migration.

CREATE TABLE IF NOT EXISTS users (
  id uuid PRIMARY KEY,
  apple_id text NOT NULL UNIQUE,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS teas (
  id uuid PRIMARY KEY,
  name text NOT NULL,
  type text NOT NULL CHECK (type IN ('tea','herb','coffee','other')),
  description text,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS teas_name_prefix_idx ON teas (lower(name) text_pattern_ops);

CREATE TABLE IF NOT EXISTS tag_categories (
  id uuid PRIMARY KEY,
  name text NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS tags (
  id uuid PRIMARY KEY,
  name text NOT NULL,
  color text NOT NULL,
  category_id uuid NOT NULL REFERENCES tag_categories(id) ON DELETE RESTRICT
);
CREATE UNIQUE INDEX IF NOT EXISTS tags_category_name_uq ON tags (category_id, lower(name));
CREATE INDEX IF NOT EXISTS tags_category_idx ON tags (category_id);

CREATE TABLE IF NOT EXISTS tea_tags (
  tea_id uuid NOT NULL REFERENCES teas(id) ON DELETE CASCADE,
  tag_id uuid NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
  PRIMARY KEY (tea_id, tag_id)
);
CREATE INDEX IF NOT EXISTS tea_tags_tag_idx ON tea_tags (tag_id);

CREATE TABLE IF NOT EXISTS qr_records (
  id uuid PRIMARY KEY,
  tea_id uuid NOT NULL REFERENCES teas(id) ON DELETE CASCADE,
  boiling_temp int NOT NULL,
  expiration_date timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS qr_records_tea_idx ON qr_records (tea_id);
CREATE INDEX IF NOT EXISTS qr_records_exp_idx ON qr_records (expiration_date);
-- Likely filter criterion during brewing suggestions/search
CREATE INDEX IF NOT EXISTS qr_records_boiling_temp_idx ON qr_records (boiling_temp);

CREATE TABLE IF NOT EXISTS collections (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS collections_user_idx ON collections (user_id);

CREATE TABLE IF NOT EXISTS collection_qr_items (
  collection_id uuid NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
  qr_id uuid NOT NULL REFERENCES qr_records(id) ON DELETE CASCADE,
  PRIMARY KEY (collection_id, qr_id)
);
CREATE INDEX IF NOT EXISTS collection_qr_items_qr_idx ON collection_qr_items (qr_id);

CREATE TABLE IF NOT EXISTS devices (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (token)
);
CREATE INDEX IF NOT EXISTS devices_user_idx ON devices (user_id);

CREATE TABLE IF NOT EXISTS notifications (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  type smallint NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS notifications_user_created_idx ON notifications (user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS consumptions (
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  ts timestamptz NOT NULL,
  tea_id uuid NOT NULL REFERENCES teas(id) ON DELETE CASCADE,
  PRIMARY KEY (user_id, ts, tea_id)
);
CREATE INDEX IF NOT EXISTS consumptions_user_ts_desc_idx ON consumptions (user_id, ts DESC);
`

type teaRow struct {
	id          uuid.UUID
	name, ttype sql.NullString
	desc        sql.NullString
}

// Backfill tool skeleton: streams data from FDB and inserts into Postgres in batches.
// Env:
//
//	DATABASEPATH - path to FoundationDB cluster file (optional; default system)
//	PG_DSN       - Postgres DSN (postgres://...)
func main() {
	ctx := context.Background()

	pgDSN := os.Getenv("PG_DSN")
	if pgDSN == "" {
		log.Printf("PG_DSN is required")
		return
	}

	// Init FDB
	foundationdb.MustAPIVersion(fdbAPIVersion)

	// Prefer explicit cluster file via env to avoid relying on container CWD/defaults
	clusterPath := os.Getenv("DATABASEPATH")
	if clusterPath == "" {
		clusterPath = os.Getenv("FDB_CLUSTER_FILE")
	}

	var (
		fdbDB foundationdb.Database
		err   error
	)
	if clusterPath != "" {
		log.Printf("opening FDB using cluster file: %s", clusterPath)
		fdbDB, err = foundationdb.OpenDatabase(clusterPath)
	} else {
		log.Printf("opening FDB using default search path (no cluster env provided)")
		fdbDB, err = foundationdb.OpenDefault()
	}
	if err != nil {
		log.Printf("open FDB: %v", err)
		return
	}

	// Wrap with our thin client
	db := fdbclient.NewDatabase(fdbDB)
	kb := key_builder.NewBuilder()

	// Connect Postgres via pgx stdlib
	pg, err := sql.Open("pgx", pgDSN)
	if err != nil {
		log.Printf("open postgres: %v", err)
		return
	}

	defer func() {
		if cerr := pg.Close(); cerr != nil {
			log.Printf("close postgres: %v", cerr)
		}
	}()

	// Ensure critical base tables exist before running the full schema to avoid FK ordering issues
	if err := ensureUsersTable(ctx, pg); err != nil {
		log.Printf("ensure users: %v", err)
		return
	}

	// Ensure schema exists (idempotent)
	if err := ensurePostgresSchema(ctx, pg); err != nil {
		log.Printf("ensure schema: %v", err)
		return
	}

	if err := backfillUsers(ctx, db, kb, pg); err != nil {
		log.Printf("backfill users: %v", err)
		return
	}

	if err := backfillTeas(ctx, db, kb, pg); err != nil {
		log.Printf("backfill teas: %v", err)
		return
	}

	if err := backfillConsumptions(ctx, db, kb, pg); err != nil {
		log.Printf("backfill consumptions: %v", err)
		return
	}

	log.Println("backfill completed")
}

// backfillTeas streams teas from FDB and writes them to Postgres.
//
//nolint:gocyclo // linear streaming with guarded branches for robustness; acceptable in a batch tool
func backfillTeas(ctx context.Context, db fdbclient.Database, kb key_builder.Builder, pg *sql.DB) error {
	tr, err := db.NewTransaction(ctx)
	if err != nil {
		return fmt.Errorf("backfillTeas: new transaction: %w", err)
	}

	pr, err := foundationdb.PrefixRange(kb.Records())
	if err != nil {
		return fmt.Errorf("backfillTeas: prefix range: %w", err)
	}

	it := tr.GetIterator(pr)

	batch := make([]teaRow, 0, teaBatchSize)

	for it.Advance() {
		kv, err := it.Get()
		if err != nil {
			return fmt.Errorf("backfillTeas: iterator get: %w", err)
		}

		id, err := uuid.FromBytes(kv.Key[1:])
		if err != nil {
			continue
		}

		var td encoder.TeaData
		if err := td.Decode(kv.Value); err != nil {
			continue
		}

		batch = append(batch, teaRow{
			id:    id,
			name:  sql.NullString{String: td.Name, Valid: true},
			ttype: sql.NullString{String: td.Type, Valid: true},
			desc:  sql.NullString{String: td.Description, Valid: true},
		})

		if len(batch) >= teaBatchSize {
			if err := insertTeas(ctx, pg, batch); err != nil {
				return fmt.Errorf("backfillTeas: insert batch: %w", err)
			}

			batch = batch[:0]
		}
	}

	if len(batch) > 0 {
		if err := insertTeas(ctx, pg, batch); err != nil {
			return fmt.Errorf("backfillTeas: insert tail: %w", err)
		}
	}

	return nil
}

func insertTeas(ctx context.Context, pg *sql.DB, batch []teaRow) error {
	// simple INSERT .. ON CONFLICT .. DO UPDATE batching (idempotent)
	for _, r := range batch {
		if _, err := pg.ExecContext(
			ctx,
			"INSERT INTO teas (id, name, type, description) VALUES ($1,$2,$3,$4) ON CONFLICT (id) DO UPDATE SET name=EXCLUDED.name, type=EXCLUDED.type, description=EXCLUDED.description",
			r.id, r.name, r.ttype, r.desc,
		); err != nil {
			return fmt.Errorf("insertTeas: exec: %w", err)
		}
	}

	return nil
}

func ensurePostgresSchema(ctx context.Context, pg *sql.DB) error {
	// Execute the embedded schema in a simple way: split by ';' and run statements
	parts := strings.Split(pgSchema, ";")
	for _, p := range parts {
		stmt := strings.TrimSpace(p)
		if stmt == "" || strings.HasPrefix(stmt, "--") {
			continue
		}
		if _, err := pg.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("ensure schema exec: %w", err)
		}
	}
	log.Printf("postgres schema ensured (idempotent)")
	return nil
}

// ensureUsersTable creates the users table early to satisfy downstream FKs in case
// the main schema execution encounters ordering issues in certain environments.
func ensureUsersTable(ctx context.Context, pg *sql.DB) error {
	ddl := `CREATE TABLE IF NOT EXISTS users (
  id uuid PRIMARY KEY,
  apple_id text NOT NULL UNIQUE,
  created_at timestamptz NOT NULL DEFAULT now()
);`
	if _, err := pg.ExecContext(ctx, ddl); err != nil {
		return fmt.Errorf("ensure users exec: %w", err)
	}
	return nil
}

func backfillUsers(ctx context.Context, db fdbclient.Database, kb key_builder.Builder, pg *sql.DB) error {
	tr, err := db.NewTransaction(ctx)
	if err != nil {
		return fmt.Errorf("backfillUsers: new transaction: %w", err)
	}

	pr, err := foundationdb.PrefixRange(kb.Users())
	if err != nil {
		return fmt.Errorf("backfillUsers: prefix range: %w", err)
	}

	kvs, err := tr.GetRange(pr)
	if err != nil {
		return fmt.Errorf("backfillUsers: get range: %w", err)
	}

	for _, kv := range kvs {
		id, err := uuid.FromBytes(kv.Key[1:])
		if err != nil {
			continue
		}

		var u encoder.User
		if err := u.Decode(kv.Value); err != nil {
			continue
		}

		// Insert user with id and apple_id
		if _, err := pg.ExecContext(
			ctx,
			"INSERT INTO users (id, apple_id) VALUES ($1,$2) ON CONFLICT (id) DO UPDATE SET apple_id=EXCLUDED.apple_id",
			id, sql.NullString{String: u.AppleID, Valid: u.AppleID != ""},
		); err != nil {
			return fmt.Errorf("backfillUsers: insert: %w", err)
		}
	}

	return nil
}

func backfillConsumptions(_ context.Context, _ fdbclient.Database, kb key_builder.Builder, _ *sql.DB) error { // revive:disable-line:unused-parameter
	_ = kb.ConsumptionByUserID(uuid.Nil)
	// In a full implementation, iterate users and scan per-user consumption prefix.
	return nil
}
