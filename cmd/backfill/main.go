// Package main contains the FDB→Postgres backfill utility.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"

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

	fdb, err := foundationdb.OpenDefault()
	if err != nil {
		log.Printf("open FDB: %v", err)
		return
	}

	// Wrap with our thin client
	db := fdbclient.NewDatabase(fdb)
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
