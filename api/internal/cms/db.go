package cms

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type Store struct {
	db        *sql.DB
	dataDir   string
	uploadDir string
}

func Open(dataDir, uploadDir string) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		return nil, err
	}
	dbPath := filepath.Join(dataDir, "cms.db")
	db, err := sql.Open("sqlite", dbPath+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db, dataDir: dataDir, uploadDir: uploadDir}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) DB() *sql.DB         { return s.db }
func (s *Store) DataDir() string     { return s.dataDir }
func (s *Store) UploadDir() string   { return s.uploadDir }

func (s *Store) migrate() error {
	schema := `
CREATE TABLE IF NOT EXISTS site_settings (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  site_name TEXT NOT NULL DEFAULT '',
  logo_html TEXT NOT NULL DEFAULT '',
  description TEXT NOT NULL DEFAULT '',
  og_image TEXT NOT NULL DEFAULT '',
  instagram_url TEXT NOT NULL DEFAULT '',
  behance_url TEXT NOT NULL DEFAULT '',
  linkedin_url TEXT NOT NULL DEFAULT '',
  copyright TEXT NOT NULL DEFAULT '',
  canonical_base TEXT NOT NULL DEFAULT '',
  updated_at TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS pages (
  id TEXT PRIMARY KEY,
  slug TEXT NOT NULL UNIQUE,
  title TEXT NOT NULL,
  theme TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'draft',
  sort_order INTEGER NOT NULL DEFAULT 0,
  meta_description TEXT NOT NULL DEFAULT '',
  og_image TEXT NOT NULL DEFAULT '',
  is_homepage INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS blocks (
  id TEXT PRIMARY KEY,
  page_id TEXT NOT NULL REFERENCES pages(id) ON DELETE CASCADE,
  type TEXT NOT NULL,
  sort_order INTEGER NOT NULL DEFAULT 0,
  data_json TEXT NOT NULL DEFAULT '{}',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS media (
  id TEXT PRIMARY KEY,
  filename TEXT NOT NULL,
  original_name TEXT NOT NULL DEFAULT '',
  url TEXT NOT NULL,
  title TEXT NOT NULL DEFAULT '',
  alt TEXT NOT NULL DEFAULT '',
  kind TEXT NOT NULL DEFAULT '',
  mime TEXT NOT NULL DEFAULT '',
  size_bytes INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS nav (
  id TEXT PRIMARY KEY,
  label TEXT NOT NULL,
  href TEXT NOT NULL DEFAULT '',
  page_id TEXT NOT NULL DEFAULT '',
  parent_id TEXT NOT NULL DEFAULT '',
  sort_order INTEGER NOT NULL DEFAULT 0,
  kind TEXT NOT NULL DEFAULT 'link',
  visible INTEGER NOT NULL DEFAULT 1
);

CREATE TABLE IF NOT EXISTS publish_history (
  id TEXT PRIMARY KEY,
  created_at TEXT NOT NULL,
  note TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'ok',
  detail_json TEXT NOT NULL DEFAULT '{}'
);

CREATE TABLE IF NOT EXISTS templates (
  id TEXT PRIMARY KEY,
  theme TEXT NOT NULL,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  allowed_blocks_json TEXT NOT NULL DEFAULT '[]',
  default_blocks_json TEXT NOT NULL DEFAULT '[]',
  is_system INTEGER NOT NULL DEFAULT 0,
  sort_order INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_blocks_page ON blocks(page_id, sort_order);
CREATE INDEX IF NOT EXISTS idx_nav_sort ON nav(sort_order);
CREATE INDEX IF NOT EXISTS idx_pages_sort ON pages(sort_order);
CREATE INDEX IF NOT EXISTS idx_templates_sort ON templates(sort_order);
`
	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}
