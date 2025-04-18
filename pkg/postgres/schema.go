package postgres

import (
	"context"
	"database/sql"
)

// Schema contains the SQL statements to create the database schema
var Schema = []string{
	// Users table
	`CREATE TABLE IF NOT EXISTS users (
		id BYTEA PRIMARY KEY,
		apple_id TEXT NOT NULL UNIQUE
	)`,

	// Tea records table
	`CREATE TABLE IF NOT EXISTS tea_records (
		id BYTEA PRIMARY KEY,
		name TEXT NOT NULL,
		type INTEGER NOT NULL DEFAULT 0,
		description TEXT,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	)`,

	// Tea record name index
	`CREATE INDEX IF NOT EXISTS tea_records_name_idx ON tea_records (name)`,

	// QR codes table
	`CREATE TABLE IF NOT EXISTS qr_codes (
		id BYTEA PRIMARY KEY,
		tea_id BYTEA NOT NULL REFERENCES tea_records(id) ON DELETE CASCADE,
		bowling_temp INTEGER,
		expiration_date TIMESTAMP WITH TIME ZONE
	)`,

	// Version table
	`CREATE TABLE IF NOT EXISTS version (
		id INTEGER PRIMARY KEY DEFAULT 1,
		version INTEGER NOT NULL
	)`,

	// Tag categories table
	`CREATE TABLE IF NOT EXISTS tag_categories (
		id BYTEA PRIMARY KEY,
		name TEXT NOT NULL UNIQUE
	)`,

	// Tags table
	`CREATE TABLE IF NOT EXISTS tags (
		id BYTEA PRIMARY KEY,
		name TEXT NOT NULL,
		color TEXT NOT NULL,
		category_id BYTEA NOT NULL REFERENCES tag_categories(id) ON DELETE CASCADE,
		UNIQUE(name, category_id)
	)`,

	// Tea-Tag relationship table
	`CREATE TABLE IF NOT EXISTS tea_tags (
		tea_id BYTEA NOT NULL REFERENCES tea_records(id) ON DELETE CASCADE,
		tag_id BYTEA NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
		PRIMARY KEY (tea_id, tag_id)
	)`,

	// Collections table
	`CREATE TABLE IF NOT EXISTS collections (
		id BYTEA PRIMARY KEY,
		name TEXT NOT NULL,
		user_id BYTEA NOT NULL REFERENCES users(id) ON DELETE CASCADE
	)`,

	// Collection-Tea relationship table
	`CREATE TABLE IF NOT EXISTS collection_teas (
		collection_id BYTEA NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
		tea_id BYTEA NOT NULL REFERENCES tea_records(id) ON DELETE CASCADE,
		PRIMARY KEY (collection_id, tea_id)
	)`,

	// Devices table
	`CREATE TABLE IF NOT EXISTS devices (
		id BYTEA PRIMARY KEY,
		token TEXT NOT NULL,
		user_id BYTEA REFERENCES users(id) ON DELETE SET NULL
	)`,

	// Notifications table
	`CREATE TABLE IF NOT EXISTS notifications (
		id BYTEA PRIMARY KEY,
		user_id BYTEA NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		type INTEGER NOT NULL,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	)`,
}

// InitSchema initializes the database schema
func InitSchema(ctx context.Context, db *sql.DB) error {
	for _, stmt := range Schema {
		_, err := db.ExecContext(ctx, stmt)
		if err != nil {
			return err
		}
	}
	return nil
}
