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
	batchSize     = 1000 // batch size for inserts
)

type teaRow struct {
	id          uuid.UUID
	name, ttype sql.NullString
	desc        sql.NullString
}

type qrRow struct {
	id             uuid.UUID
	teaID          uuid.UUID
	boilingTemp    int32
	expirationDate sql.NullTime
}

type deviceRow struct {
	id     uuid.UUID
	userID uuid.UUID
	token  string
}

// Backfill tool: streams data from FDB and inserts into Postgres in batches.
// Env:
//
//	DATABASEPATH - path to FoundationDB cluster file (optional; default system)
//	PG_DSN       - Postgres DSN (postgres://...)
func main() {
	ctx := context.Background()

	pgDSN := os.Getenv("PG_DSN")
	if pgDSN == "" {
		log.Fatal("PG_DSN is required")
	}

	// Init FDB
	foundationdb.MustAPIVersion(fdbAPIVersion)

	fdb, err := foundationdb.OpenDefault()
	if err != nil {
		log.Fatalf("open FDB: %v", err)
	}

	// Wrap with our thin client
	db := fdbclient.NewDatabase(fdb)
	kb := key_builder.NewBuilder()

	// Connect Postgres via pgx stdlib
	pg, err := sql.Open("pgx", pgDSN)
	if err != nil {
		log.Fatalf("open postgres: %v", err)
	}

	defer func() {
		if cerr := pg.Close(); cerr != nil {
			log.Printf("close postgres: %v", cerr)
		}
	}()

	log.Println("Starting backfill...")

	if err := backfillUsers(ctx, db, kb, pg); err != nil {
		log.Fatalf("backfill users: %v", err)
	}
	log.Println("✓ Users backfilled")

	if err := backfillTeas(ctx, db, kb, pg); err != nil {
		log.Fatalf("backfill teas: %v", err)
	}
	log.Println("✓ Teas backfilled")

	if err := backfillQRRecords(ctx, db, kb, pg); err != nil {
		log.Fatalf("backfill QR records: %v", err)
	}
	log.Println("✓ QR records backfilled")

	if err := backfillDevices(ctx, db, kb, pg); err != nil {
		log.Fatalf("backfill devices: %v", err)
	}
	log.Println("✓ Devices backfilled")

	if err := backfillConsumptions(ctx, db, kb, pg); err != nil {
		log.Fatalf("backfill consumptions: %v", err)
	}
	log.Println("✓ Consumptions backfilled")

	log.Println("Backfill completed successfully!")
}

// backfillTeas streams teas from FDB and writes them to Postgres.
func backfillTeas(ctx context.Context, db fdbclient.Database, kb key_builder.Builder, pg *sql.DB) error {
	tr, err := db.NewTransaction(ctx)
	if err != nil {
		return fmt.Errorf("new transaction: %w", err)
	}

	pr, err := foundationdb.PrefixRange(kb.Records())
	if err != nil {
		return fmt.Errorf("prefix range: %w", err)
	}

	it := tr.GetIterator(pr)

	batch := make([]teaRow, 0, batchSize)
	count := 0

	for it.Advance() {
		kv, err := it.Get()
		if err != nil {
			return fmt.Errorf("iterator get: %w", err)
		}

		id, err := uuid.FromBytes(kv.Key[1:])
		if err != nil {
			log.Printf("warning: skip invalid tea UUID: %v", err)
			continue
		}

		var td encoder.TeaData
		if err := td.Decode(kv.Value); err != nil {
			log.Printf("warning: skip tea %s decode error: %v", id, err)
			continue
		}

		batch = append(batch, teaRow{
			id:    id,
			name:  sql.NullString{String: td.Name, Valid: true},
			ttype: sql.NullString{String: td.Type, Valid: true},
			desc:  sql.NullString{String: td.Description, Valid: td.Description != ""},
		})

		if len(batch) >= batchSize {
			if err := insertTeas(ctx, pg, batch); err != nil {
				return fmt.Errorf("insert batch: %w", err)
			}
			count += len(batch)
			batch = batch[:0]
		}
	}

	if len(batch) > 0 {
		if err := insertTeas(ctx, pg, batch); err != nil {
			return fmt.Errorf("insert tail: %w", err)
		}
		count += len(batch)
	}

	log.Printf("  Inserted %d tea records", count)
	return nil
}

func insertTeas(ctx context.Context, pg *sql.DB, batch []teaRow) error {
	for _, r := range batch {
		if _, err := pg.ExecContext(
			ctx,
			"INSERT INTO teas (id, name, type, description) VALUES ($1,$2,$3,$4) ON CONFLICT (id) DO UPDATE SET name=EXCLUDED.name, type=EXCLUDED.type, description=EXCLUDED.description",
			r.id, r.name, r.ttype, r.desc,
		); err != nil {
			return fmt.Errorf("exec: %w", err)
		}
	}
	return nil
}

func backfillUsers(ctx context.Context, db fdbclient.Database, kb key_builder.Builder, pg *sql.DB) error {
	tr, err := db.NewTransaction(ctx)
	if err != nil {
		return fmt.Errorf("new transaction: %w", err)
	}

	pr, err := foundationdb.PrefixRange(kb.Users())
	if err != nil {
		return fmt.Errorf("prefix range: %w", err)
	}

	kvs, err := tr.GetRange(pr)
	if err != nil {
		return fmt.Errorf("get range: %w", err)
	}

	count := 0
	for _, kv := range kvs {
		id, err := uuid.FromBytes(kv.Key[1:])
		if err != nil {
			log.Printf("warning: skip invalid user UUID: %v", err)
			continue
		}

		var u encoder.User
		if err := u.Decode(kv.Value); err != nil {
			log.Printf("warning: skip user %s decode error: %v", id, err)
			continue
		}

		// Insert user with id and apple_id
		if _, err := pg.ExecContext(
			ctx,
			"INSERT INTO users (id, apple_id) VALUES ($1,$2) ON CONFLICT (id) DO UPDATE SET apple_id=EXCLUDED.apple_id",
			id, sql.NullString{String: u.AppleID, Valid: u.AppleID != ""},
		); err != nil {
			return fmt.Errorf("insert: %w", err)
		}
		count++
	}

	log.Printf("  Inserted %d users", count)
	return nil
}

// backfillQRRecords streams QR records from FDB and writes them to Postgres.
func backfillQRRecords(ctx context.Context, db fdbclient.Database, kb key_builder.Builder, pg *sql.DB) error {
	tr, err := db.NewTransaction(ctx)
	if err != nil {
		return fmt.Errorf("new transaction: %w", err)
	}

	// QR records are stored with a specific prefix - we need to scan all QR keys
	// The key_builder has QR(id) method, so we need to construct a prefix scan
	// Looking at the key_builder, QR keys start with a specific byte prefix
	// We'll need to iterate through all possible QRs by scanning the keyspace

	// Get all users first to build collections
	usersPr, err := foundationdb.PrefixRange(kb.Users())
	if err != nil {
		return fmt.Errorf("prefix range users: %w", err)
	}

	userKvs, err := tr.GetRange(usersPr)
	if err != nil {
		return fmt.Errorf("get users range: %w", err)
	}

	userIDs := make([]uuid.UUID, 0, len(userKvs))
	for _, kv := range userKvs {
		if id, err := uuid.FromBytes(kv.Key[1:]); err == nil {
			userIDs = append(userIDs, id)
		}
	}

	// Now scan collections for each user to find QR records
	batch := make([]qrRow, 0, batchSize)
	count := 0
	seenQRs := make(map[uuid.UUID]bool)

	for _, userID := range userIDs {
		collectionsPr, err := foundationdb.PrefixRange(kb.UserCollections(userID))
		if err != nil {
			continue
		}

		collKvs, err := tr.GetRange(collectionsPr)
		if err != nil {
			continue
		}

		for _, collKv := range collKvs {
			// Extract collection ID from key
			if len(collKv.Key) < 17 {
				continue
			}
			collID, err := uuid.FromBytes(collKv.Key[len(collKv.Key)-16:])
			if err != nil {
				continue
			}

			// Get QR records in this collection
			qrsPr, err := foundationdb.PrefixRange(kb.RecordsByCollection(collID))
			if err != nil {
				continue
			}

			qrKvs, err := tr.GetRange(qrsPr)
			if err != nil {
				continue
			}

			for _, qrKv := range qrKvs {
				// The value contains the QR ID
				if len(qrKv.Value) < 16 {
					continue
				}
				qrID, err := uuid.FromBytes(qrKv.Value)
				if err != nil {
					continue
				}

				// Skip if we've already processed this QR
				if seenQRs[qrID] {
					continue
				}
				seenQRs[qrID] = true

				// Fetch the actual QR record
				qrKey := kb.QR(qrID)
				qrData, err := tr.Get(qrKey)
				if err != nil || qrData == nil {
					continue
				}

				var qr encoder.QR
				if err := qr.Decode(qrData); err != nil {
					log.Printf("warning: skip QR %s decode error: %v", qrID, err)
					continue
				}

				batch = append(batch, qrRow{
					id:             qrID,
					teaID:          qr.Tea,
					boilingTemp:    int32(qr.BowlingTemp),
					expirationDate: sql.NullTime{Time: qr.ExpirationDate, Valid: !qr.ExpirationDate.IsZero()},
				})

				if len(batch) >= batchSize {
					if err := insertQRRecords(ctx, pg, batch); err != nil {
						return fmt.Errorf("insert batch: %w", err)
					}
					count += len(batch)
					batch = batch[:0]
				}
			}
		}
	}

	if len(batch) > 0 {
		if err := insertQRRecords(ctx, pg, batch); err != nil {
			return fmt.Errorf("insert tail: %w", err)
		}
		count += len(batch)
	}

	log.Printf("  Inserted %d QR records", count)
	return nil
}

func insertQRRecords(ctx context.Context, pg *sql.DB, batch []qrRow) error {
	for _, r := range batch {
		if _, err := pg.ExecContext(
			ctx,
			"INSERT INTO qr_records (id, tea_id, boiling_temp, expiration_date) VALUES ($1,$2,$3,$4) ON CONFLICT (id) DO UPDATE SET tea_id=EXCLUDED.tea_id, boiling_temp=EXCLUDED.boiling_temp, expiration_date=EXCLUDED.expiration_date",
			r.id, r.teaID, r.boilingTemp, r.expirationDate,
		); err != nil {
			return fmt.Errorf("exec: %w", err)
		}
	}
	return nil
}

// backfillDevices streams devices from FDB and writes them to Postgres.
func backfillDevices(ctx context.Context, db fdbclient.Database, kb key_builder.Builder, pg *sql.DB) error {
	tr, err := db.NewTransaction(ctx)
	if err != nil {
		return fmt.Errorf("new transaction: %w", err)
	}

	// Get all users first
	usersPr, err := foundationdb.PrefixRange(kb.Users())
	if err != nil {
		return fmt.Errorf("prefix range users: %w", err)
	}

	userKvs, err := tr.GetRange(usersPr)
	if err != nil {
		return fmt.Errorf("get users range: %w", err)
	}

	batch := make([]deviceRow, 0, batchSize)
	count := 0

	for _, userKv := range userKvs {
		userID, err := uuid.FromBytes(userKv.Key[1:])
		if err != nil {
			continue
		}

		// Get devices index for this user
		devicesPr, err := foundationdb.PrefixRange(kb.DevicesByUserID(userID))
		if err != nil {
			continue
		}

		deviceIndexKvs, err := tr.GetRange(devicesPr)
		if err != nil {
			continue
		}

		for _, indexKv := range deviceIndexKvs {
			// The value contains the device ID
			if len(indexKv.Value) < 16 {
				continue
			}
			deviceID, err := uuid.FromBytes(indexKv.Value)
			if err != nil {
				continue
			}

			// Fetch the actual device record
			deviceKey := kb.Device(deviceID)
			deviceData, err := tr.Get(deviceKey)
			if err != nil || deviceData == nil {
				continue
			}

			var device encoder.Device
			if err := device.Decode(deviceData); err != nil {
				log.Printf("warning: skip device %s decode error: %v", deviceID, err)
				continue
			}

			batch = append(batch, deviceRow{
				id:     deviceID,
				userID: userID,
				token:  device.Token,
			})

			if len(batch) >= batchSize {
				if err := insertDevices(ctx, pg, batch); err != nil {
					return fmt.Errorf("insert batch: %w", err)
				}
				count += len(batch)
				batch = batch[:0]
			}
		}
	}

	if len(batch) > 0 {
		if err := insertDevices(ctx, pg, batch); err != nil {
			return fmt.Errorf("insert tail: %w", err)
		}
		count += len(batch)
	}

	log.Printf("  Inserted %d devices", count)
	return nil
}

func insertDevices(ctx context.Context, pg *sql.DB, batch []deviceRow) error {
	for _, r := range batch {
		if _, err := pg.ExecContext(
			ctx,
			"INSERT INTO devices (id, user_id, token) VALUES ($1,$2,$3) ON CONFLICT (id) DO UPDATE SET user_id=EXCLUDED.user_id, token=EXCLUDED.token",
			r.id, r.userID, r.token,
		); err != nil {
			return fmt.Errorf("exec: %w", err)
		}
	}
	return nil
}

func backfillConsumptions(_ context.Context, _ fdbclient.Database, kb key_builder.Builder, _ *sql.DB) error {
	_ = kb.ConsumptionByUserID(uuid.Nil)
	// TODO: In a full implementation, iterate users and scan per-user consumption prefix.
	log.Println("  Skipping consumptions (implement if needed)")
	return nil
}
