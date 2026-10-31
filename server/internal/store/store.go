// Package store is a SQLite-backed key/value store.
//
// Schema is intentionally identical to the Python version so an existing
// data.db can be reused as-is.
package store

import (
	"database/sql"
	"encoding/json"
	"os"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	mu       sync.RWMutex
	db       *sql.DB
	OnChange func(key string)
}

const schema = `
CREATE TABLE IF NOT EXISTS kv (
  k TEXT PRIMARY KEY,
  v TEXT NOT NULL,
  exp INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_kv_exp ON kv(exp);
`

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	// the db holds API tokens and ssh passwords: keep it owner-only
	_ = os.Chmod(path, 0o600)
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		return nil, err
	}
	_, _ = db.Exec("PRAGMA journal_mode=WAL;")
	_, _ = db.Exec("PRAGMA synchronous=NORMAL;")
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

// Snapshot writes a consistent copy of the database to path (VACUUM INTO).
func (s *Store) Snapshot(path string) error {
	_ = os.Remove(path)
	if _, err := s.db.Exec("VACUUM INTO ?", path); err != nil {
		return err
	}
	_ = os.Chmod(path, 0o600)
	return nil
}

func (s *Store) notify(key string) {
	if s.OnChange != nil {
		s.OnChange(key)
	}
}

// Get returns the value or "" when missing/expired.
func (s *Store) Get(key string) string {
	v, _ := s.GetOK(key)
	return v
}

func (s *Store) GetOK(key string) (string, bool) {
	s.mu.RLock()
	var v string
	var exp int64
	err := s.db.QueryRow("SELECT v, exp FROM kv WHERE k = ?", key).Scan(&v, &exp)
	s.mu.RUnlock()
	if err == sql.ErrNoRows || err != nil {
		return "", false
	}
	if exp != 0 && exp <= time.Now().Unix() {
		s.Delete(key)
		return "", false
	}
	return v, true
}

func (s *Store) Put(key, val string, ttlSec int) {
	var exp int64
	if ttlSec > 0 {
		exp = time.Now().Unix() + int64(ttlSec)
	}
	s.mu.Lock()
	_, _ = s.db.Exec("INSERT INTO kv(k,v,exp) VALUES(?,?,?) "+
		"ON CONFLICT(k) DO UPDATE SET v=excluded.v, exp=excluded.exp",
		key, val, exp)
	s.mu.Unlock()
	s.notify(key)
}

func (s *Store) Delete(key string) {
	s.mu.Lock()
	_, _ = s.db.Exec("DELETE FROM kv WHERE k = ?", key)
	s.mu.Unlock()
	s.notify(key)
}

// Purge removes expired keys.
func (s *Store) Purge() {
	s.mu.Lock()
	_, _ = s.db.Exec("DELETE FROM kv WHERE exp != 0 AND exp <= ?", time.Now().Unix())
	s.mu.Unlock()
}

func (s *Store) GetJSON(key string, dst any) bool {
	v, ok := s.GetOK(key)
	if !ok || v == "" {
		return false
	}
	return json.Unmarshal([]byte(v), dst) == nil
}

func (s *Store) PutJSON(key string, obj any, ttlSec int) {
	b, err := json.Marshal(obj)
	if err != nil {
		return
	}
	s.Put(key, string(b), ttlSec)
}
