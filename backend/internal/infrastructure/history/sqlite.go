// Package history persists complete immutable analysis snapshots.
package history

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/gob"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"finance.local/amlens/internal/application"
	_ "modernc.org/sqlite"
)

type Store struct{ db *sql.DB }

func Open(ctx context.Context, path string, demo ...bool) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db}
	if err = s.initialize(ctx); err != nil {
		db.Close()
		return nil, err
	}
	mode := "workspace"
	if len(demo) > 0 && demo[0] {
		mode = "synthetic-demo-v1"
	}
	if _, err = db.ExecContext(ctx, "INSERT INTO metadata VALUES ('mode', ?) ON CONFLICT(key) DO NOTHING", mode); err != nil {
		db.Close()
		return nil, err
	}
	var existing string
	if err = db.QueryRowContext(ctx, "SELECT value FROM metadata WHERE key='mode'").Scan(&existing); err != nil {
		db.Close()
		return nil, err
	}
	if existing != mode {
		db.Close()
		return nil, errors.New("database belongs to a different operating mode; use a separate database")
	}
	return s, nil
}
func (s *Store) Close() error { return s.db.Close() }
func (s *Store) initialize(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, "PRAGMA busy_timeout=5000"); err != nil {
		return err
	}
	var version int
	if err := s.db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	if version != 0 && version != 1 {
		return fmt.Errorf("unsupported analysis database version: %d", version)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, statement := range []string{
		"CREATE TABLE IF NOT EXISTS analyses (id TEXT PRIMARY KEY, created_at TEXT NOT NULL, nodes INTEGER NOT NULL, edges INTEGER NOT NULL, snapshot BLOB NOT NULL)",
		"CREATE TABLE IF NOT EXISTS active (singleton INTEGER PRIMARY KEY CHECK(singleton=1), analysis_id TEXT NOT NULL)",
		"CREATE TABLE IF NOT EXISTS metadata (key TEXT PRIMARY KEY, value TEXT NOT NULL)",
		"PRAGMA user_version=1",
	} {
		if _, err = tx.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func decode(data []byte) (*application.Snapshot, error) {
	var result application.Snapshot
	// Gob includes the internal evidence fields omitted from the public JSON API,
	// and preserves decimal amounts and int64 identifiers exactly.
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&result); err != nil {
		return nil, err
	}
	if result.Analysis.ID == "" || len(result.Exports) != 3 {
		return nil, errors.New("invalid stored analysis")
	}
	return &result, nil
}
func (s *Store) Save(ctx context.Context, snapshot *application.Snapshot) error {
	var data bytes.Buffer
	if err := gob.NewEncoder(&data).Encode(snapshot); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	a := snapshot.Analysis
	if _, err = tx.ExecContext(ctx, "INSERT INTO analyses VALUES (?, ?, ?, ?, ?)", a.ID, time.Now().UTC().Format(time.RFC3339Nano), a.Summary.Nodes, a.Summary.Edges, data.Bytes()); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO active VALUES (1, ?) ON CONFLICT(singleton) DO UPDATE SET analysis_id=excluded.analysis_id", a.ID); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) Current(ctx context.Context) (*application.Snapshot, error) {
	var id string
	err := s.db.QueryRowContext(ctx, "SELECT analysis_id FROM active WHERE singleton=1").Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var data []byte
	if err = s.db.QueryRowContext(ctx, "SELECT snapshot FROM analyses WHERE id=?", id).Scan(&data); err != nil {
		return nil, err
	}
	return decode(data)
}
func (s *Store) List(ctx context.Context) ([]application.AnalysisRecord, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, created_at, nodes, edges FROM analyses ORDER BY created_at DESC, id DESC LIMIT 100")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []application.AnalysisRecord{}
	for rows.Next() {
		var entry application.AnalysisRecord
		if err = rows.Scan(&entry.ID, &entry.CreatedAt, &entry.Nodes, &entry.Edges); err != nil {
			return nil, err
		}
		result = append(result, entry)
	}
	return result, rows.Err()
}
func (s *Store) Activate(ctx context.Context, id string) (*application.Snapshot, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var data []byte
	err = tx.QueryRowContext(ctx, "SELECT snapshot FROM analyses WHERE id=?", id).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, application.Fail("ANALYSIS_NOT_FOUND", "Сохранённый анализ не найден")
	}
	if err != nil {
		return nil, err
	}
	snapshot, err := decode(data)
	if err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO active VALUES (1, ?) ON CONFLICT(singleton) DO UPDATE SET analysis_id=excluded.analysis_id", id); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return snapshot, nil
}
