package cms

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

var ErrNotFound = errors.New("not found")

func (s *Store) GetSettings() (SiteSettings, error) {
	var st SiteSettings
	err := s.db.QueryRow(`
SELECT site_name, logo_html, description, og_image, instagram_url, behance_url,
       linkedin_url, copyright, canonical_base, updated_at
FROM site_settings WHERE id = 1`).Scan(
		&st.SiteName, &st.LogoHTML, &st.Description, &st.OGImage,
		&st.InstagramURL, &st.BehanceURL, &st.LinkedInURL, &st.Copyright,
		&st.CanonicalBase, &st.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return SiteSettings{}, nil
	}
	return st, err
}

func (s *Store) PutSettings(st SiteSettings) (SiteSettings, error) {
	st.UpdatedAt = Now()
	_, err := s.db.Exec(`
INSERT INTO site_settings (
  id, site_name, logo_html, description, og_image, instagram_url, behance_url,
  linkedin_url, copyright, canonical_base, updated_at
) VALUES (1,?,?,?,?,?,?,?,?,?,?)
ON CONFLICT(id) DO UPDATE SET
  site_name=excluded.site_name,
  logo_html=excluded.logo_html,
  description=excluded.description,
  og_image=excluded.og_image,
  instagram_url=excluded.instagram_url,
  behance_url=excluded.behance_url,
  linkedin_url=excluded.linkedin_url,
  copyright=excluded.copyright,
  canonical_base=excluded.canonical_base,
  updated_at=excluded.updated_at
`, st.SiteName, st.LogoHTML, st.Description, st.OGImage, st.InstagramURL,
		st.BehanceURL, st.LinkedInURL, st.Copyright, st.CanonicalBase, st.UpdatedAt)
	if err != nil {
		return SiteSettings{}, err
	}
	return s.GetSettings()
}

func (s *Store) ListPages() ([]Page, error) {
	rows, err := s.db.Query(`
SELECT id, slug, title, theme, status, sort_order, meta_description, og_image,
       is_homepage, created_at, updated_at
FROM pages ORDER BY sort_order ASC, title ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Page, 0)
	for rows.Next() {
		var p Page
		var home int
		if err := rows.Scan(&p.ID, &p.Slug, &p.Title, &p.Theme, &p.Status, &p.SortOrder,
			&p.MetaDescription, &p.OGImage, &home, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		p.IsHomepage = home == 1
		p.NormalizeAliases()
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) GetPage(id string) (Page, error) {
	var p Page
	var home int
	err := s.db.QueryRow(`
SELECT id, slug, title, theme, status, sort_order, meta_description, og_image,
       is_homepage, created_at, updated_at
FROM pages WHERE id = ?`, id).Scan(
		&p.ID, &p.Slug, &p.Title, &p.Theme, &p.Status, &p.SortOrder,
		&p.MetaDescription, &p.OGImage, &home, &p.CreatedAt, &p.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Page{}, ErrNotFound
	}
	if err != nil {
		return Page{}, err
	}
	p.IsHomepage = home == 1
	blocks, err := s.ListBlocks(id)
	if err != nil {
		return Page{}, err
	}
	p.Blocks = blocks
	p.NormalizeAliases()
	return p, nil
}

func (s *Store) CreatePage(p Page) (Page, error) {
	p.NormalizeAliases()
	if p.ID == "" {
		p.ID = NewID()
	}
	if p.Status == "" {
		p.Status = "draft"
	}
	if p.Theme == "" {
		p.Theme = ThemeTextContent
	}
	if !ValidTheme(p.Theme) {
		return Page{}, fmt.Errorf("invalid theme")
	}
	now := Now()
	p.CreatedAt = now
	p.UpdatedAt = now
	home := 0
	if p.IsHomepage {
		home = 1
		if _, err := s.db.Exec(`UPDATE pages SET is_homepage = 0`); err != nil {
			return Page{}, err
		}
	}
	_, err := s.db.Exec(`
INSERT INTO pages (id, slug, title, theme, status, sort_order, meta_description, og_image, is_homepage, created_at, updated_at)
VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		p.ID, p.Slug, p.Title, p.Theme, p.Status, p.SortOrder, p.MetaDescription, p.OGImage, home, p.CreatedAt, p.UpdatedAt)
	if err != nil {
		return Page{}, err
	}
	out, err := s.GetPage(p.ID)
	if err != nil {
		return Page{}, err
	}
	out.NormalizeAliases()
	return out, nil
}

func (s *Store) PatchPage(id string, patch map[string]any) (Page, error) {
	cur, err := s.GetPage(id)
	if err != nil {
		return Page{}, err
	}
	if v, ok := patch["slug"].(string); ok {
		cur.Slug = v
	}
	if v, ok := patch["title"].(string); ok {
		cur.Title = v
	}
	if v, ok := patch["theme"].(string); ok {
		if !ValidTheme(v) {
			return Page{}, fmt.Errorf("invalid theme")
		}
		cur.Theme = v
	}
	if v, ok := patch["status"].(string); ok {
		cur.Status = v
	}
	if v, ok := patch["meta_description"].(string); ok {
		cur.MetaDescription = v
	}
	if v, ok := patch["og_image"].(string); ok {
		cur.OGImage = v
	}
	if v, ok := patch["sort_order"].(float64); ok {
		cur.SortOrder = int(v)
	}
	if v, ok := patch["is_homepage"].(bool); ok {
		cur.IsHomepage = v
	}
	cur.UpdatedAt = Now()
	home := 0
	if cur.IsHomepage {
		home = 1
		if _, err := s.db.Exec(`UPDATE pages SET is_homepage = 0 WHERE id != ?`, id); err != nil {
			return Page{}, err
		}
	}
	_, err = s.db.Exec(`
UPDATE pages SET slug=?, title=?, theme=?, status=?, sort_order=?, meta_description=?,
  og_image=?, is_homepage=?, updated_at=? WHERE id=?`,
		cur.Slug, cur.Title, cur.Theme, cur.Status, cur.SortOrder, cur.MetaDescription,
		cur.OGImage, home, cur.UpdatedAt, id)
	if err != nil {
		return Page{}, err
	}
	out, err := s.GetPage(id)
	if err != nil {
		return Page{}, err
	}
	_ = s.SyncNavForPage(out)
	return out, nil
}

// SyncNavForPage updates menu label/href for all nav rows linked to this page.
func (s *Store) SyncNavForPage(p Page) error {
	label := strings.TrimSpace(p.NavLabel)
	if label == "" {
		label = strings.TrimSpace(p.Title)
	}
	href := "/"
	if !p.IsHomepage && p.Slug != "" {
		href = "/" + strings.Trim(p.Slug, "/")
	}
	_, err := s.db.Exec(`UPDATE nav SET label = ?, href = ? WHERE page_id = ? AND page_id != ''`, label, href, p.ID)
	return err
}

func (s *Store) DeletePage(id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.Exec(`DELETE FROM pages WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	// Drop blocks for the page (no FK cascade).
	if _, err := tx.Exec(`DELETE FROM blocks WHERE page_id = ?`, id); err != nil {
		return err
	}
	// Drop menu entries linked to this page.
	if _, err := tx.Exec(`DELETE FROM nav WHERE page_id = ? AND page_id != ''`, id); err != nil {
		return err
	}
	if err := pruneEmptyNavCategoriesTx(tx); err != nil {
		return err
	}
	return tx.Commit()
}

// CleanupOrphanNav removes nav rows whose page_id no longer exists, then
// drops empty category parents. Safe to call on startup.
func (s *Store) CleanupOrphanNav() (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.Exec(`
DELETE FROM nav
WHERE page_id != ''
  AND page_id NOT IN (SELECT id FROM pages)`)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	if err := pruneEmptyNavCategoriesTx(tx); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return int(n), nil
}

// pruneEmptyNavCategoriesTx deletes category rows that have no children left.
func pruneEmptyNavCategoriesTx(tx *sql.Tx) error {
	for {
		res, err := tx.Exec(`
DELETE FROM nav
WHERE kind = 'category'
  AND id NOT IN (
    SELECT parent_id FROM (
      SELECT DISTINCT parent_id AS parent_id FROM nav WHERE parent_id != ''
    )
  )`)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			return nil
		}
	}
}

func (s *Store) ListBlocks(pageID string) ([]Block, error) {
	rows, err := s.db.Query(`
SELECT id, page_id, type, sort_order, data_json, created_at, updated_at
FROM blocks WHERE page_id = ? ORDER BY sort_order ASC`, pageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Block, 0)
	for rows.Next() {
		var b Block
		var raw string
		if err := rows.Scan(&b.ID, &b.PageID, &b.Type, &b.SortOrder, &raw, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, err
		}
		b.Data = json.RawMessage(raw)
		b.NormalizeAliases()
		out = append(out, b)
	}
	return out, rows.Err()
}

func (s *Store) ReplaceBlocks(pageID string, blocks []Block) ([]Block, error) {
	if _, err := s.GetPage(pageID); err != nil {
		return nil, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`DELETE FROM blocks WHERE page_id = ?`, pageID); err != nil {
		return nil, err
	}
	now := Now()
	for i, b := range blocks {
		if b.ID == "" {
			b.ID = NewID()
		}
		if !ValidBlockType(b.Type) {
			return nil, fmt.Errorf("invalid block type: %s", b.Type)
		}
		if len(b.Data) == 0 {
			b.Data = json.RawMessage(`{}`)
		}
		if _, err := tx.Exec(`
INSERT INTO blocks (id, page_id, type, sort_order, data_json, created_at, updated_at)
VALUES (?,?,?,?,?,?,?)`, b.ID, pageID, b.Type, i, string(b.Data), now, now); err != nil {
			return nil, err
		}
	}
	if _, err := tx.Exec(`UPDATE pages SET updated_at = ? WHERE id = ?`, now, pageID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	out, err := s.ListBlocks(pageID)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].NormalizeAliases()
	}
	return out, nil
}

func (s *Store) ListMedia() ([]Media, error) {
	return s.ListMediaFiltered("", "")
}

func (s *Store) ListMediaFiltered(kind, q string) ([]Media, error) {
	rows, err := s.db.Query(`
SELECT id, filename, original_name, url, title, alt, kind, mime, size_bytes, created_at, updated_at
FROM media ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Media, 0)
	kind = strings.TrimSpace(kind)
	q = strings.ToLower(strings.TrimSpace(q))
	for rows.Next() {
		var m Media
		if err := rows.Scan(&m.ID, &m.Filename, &m.OriginalName, &m.URL, &m.Title, &m.Alt,
			&m.Kind, &m.Mime, &m.SizeBytes, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		if kind != "" && m.Kind != kind {
			continue
		}
		if q != "" {
			hay := strings.ToLower(m.Title + " " + m.OriginalName + " " + m.Filename + " " + m.Alt)
			if !strings.Contains(hay, q) {
				continue
			}
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) GetMedia(id string) (Media, error) {
	var m Media
	err := s.db.QueryRow(`
SELECT id, filename, original_name, url, title, alt, kind, mime, size_bytes, created_at, updated_at
FROM media WHERE id = ?`, id).Scan(
		&m.ID, &m.Filename, &m.OriginalName, &m.URL, &m.Title, &m.Alt,
		&m.Kind, &m.Mime, &m.SizeBytes, &m.CreatedAt, &m.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Media{}, ErrNotFound
	}
	return m, err
}

func (s *Store) CreateMedia(m Media) (Media, error) {
	if m.ID == "" {
		m.ID = NewID()
	}
	now := Now()
	m.CreatedAt = now
	m.UpdatedAt = now
	_, err := s.db.Exec(`
INSERT INTO media (id, filename, original_name, url, title, alt, kind, mime, size_bytes, created_at, updated_at)
VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		m.ID, m.Filename, m.OriginalName, m.URL, m.Title, m.Alt, m.Kind, m.Mime, m.SizeBytes, m.CreatedAt, m.UpdatedAt)
	if err != nil {
		return Media{}, err
	}
	return s.GetMedia(m.ID)
}

func (s *Store) PatchMedia(id string, title, alt, kind *string) (Media, error) {
	cur, err := s.GetMedia(id)
	if err != nil {
		return Media{}, err
	}
	if title != nil {
		cur.Title = *title
	}
	if alt != nil {
		cur.Alt = *alt
	}
	if kind != nil {
		cur.Kind = *kind
	}
	cur.UpdatedAt = Now()
	_, err = s.db.Exec(`UPDATE media SET title=?, alt=?, kind=?, updated_at=? WHERE id=?`,
		cur.Title, cur.Alt, cur.Kind, cur.UpdatedAt, id)
	if err != nil {
		return Media{}, err
	}
	return s.GetMedia(id)
}

func (s *Store) DeleteMedia(id string) (Media, error) {
	m, err := s.GetMedia(id)
	if err != nil {
		return Media{}, err
	}
	_, err = s.db.Exec(`DELETE FROM media WHERE id = ?`, id)
	return m, err
}

func (s *Store) CountMedia() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM media`).Scan(&n)
	return n, err
}

func (s *Store) CountPages() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM pages`).Scan(&n)
	return n, err
}

func (s *Store) GetNavFlat() ([]NavItem, error) {
	rows, err := s.db.Query(`
SELECT id, label, href, page_id, parent_id, sort_order, kind, visible
FROM nav ORDER BY sort_order ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]NavItem, 0)
	for rows.Next() {
		var n NavItem
		var vis int
		if err := rows.Scan(&n.ID, &n.Label, &n.Href, &n.PageID, &n.ParentID, &n.SortOrder, &n.Kind, &vis); err != nil {
			return nil, err
		}
		n.Visible = vis == 1
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *Store) GetNavTree() ([]NavItem, error) {
	flat, err := s.GetNavFlat()
	if err != nil {
		return nil, err
	}
	// Defensive: skip links whose page was deleted but nav row remains.
	alive := map[string]struct{}{}
	rows, err := s.db.Query(`SELECT id FROM pages`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		alive[id] = struct{}{}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	byID := map[string]*NavItem{}
	roots := make([]*NavItem, 0)
	for i := range flat {
		item := flat[i]
		if item.PageID != "" {
			if _, ok := alive[item.PageID]; !ok {
				continue
			}
		}
		item.Children = []NavItem{}
		cp := item
		byID[cp.ID] = &cp
	}
	for i := range flat {
		item, ok := byID[flat[i].ID]
		if !ok {
			continue
		}
		if item.ParentID != "" {
			if parent, ok := byID[item.ParentID]; ok {
				parent.Children = append(parent.Children, *item)
				continue
			}
		}
		roots = append(roots, item)
	}
	out := make([]NavItem, 0, len(roots))
	for _, r := range roots {
		// Hide empty categories (all children were orphaned).
		if r.Kind == "category" && len(r.Children) == 0 {
			continue
		}
		out = append(out, *r)
	}
	return out, nil
}

func (s *Store) ReplaceNav(items []NavItem) ([]NavItem, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`DELETE FROM nav`); err != nil {
		return nil, err
	}
	var insert func(parentID string, list []NavItem, orderStart int) error
	insert = func(parentID string, list []NavItem, orderStart int) error {
		for i, item := range list {
			if item.ID == "" {
				item.ID = NewID()
			}
			if item.Kind == "" {
				if len(item.Children) > 0 {
					item.Kind = "category"
				} else {
					item.Kind = "link"
				}
			}
			vis := 0
			if item.Visible {
				vis = 1
			}
			ord := orderStart + i
			if item.SortOrder != 0 {
				ord = item.SortOrder
			}
			if _, err := tx.Exec(`
INSERT INTO nav (id, label, href, page_id, parent_id, sort_order, kind, visible)
VALUES (?,?,?,?,?,?,?,?)`,
				item.ID, item.Label, item.Href, item.PageID, parentID, ord, item.Kind, vis); err != nil {
				return err
			}
			if len(item.Children) > 0 {
				if err := insert(item.ID, item.Children, 0); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := insert("", items, 0); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetNavTree()
}

func (s *Store) AddPublishHistory(h PublishHistory) (PublishHistory, error) {
	if h.ID == "" {
		h.ID = NewID()
	}
	if h.CreatedAt == "" {
		h.CreatedAt = Now()
	}
	if h.Status == "" {
		h.Status = "ok"
	}
	if len(h.Detail) == 0 {
		h.Detail = json.RawMessage(`{}`)
	}
	_, err := s.db.Exec(`
INSERT INTO publish_history (id, created_at, note, status, detail_json)
VALUES (?,?,?,?,?)`, h.ID, h.CreatedAt, h.Note, h.Status, string(h.Detail))
	return h, err
}

func (s *Store) ListPublishHistory(limit int) ([]PublishHistory, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(`
SELECT id, created_at, note, status, detail_json
FROM publish_history ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]PublishHistory, 0)
	for rows.Next() {
		var h PublishHistory
		var detail string
		if err := rows.Scan(&h.ID, &h.CreatedAt, &h.Note, &h.Status, &detail); err != nil {
			return nil, err
		}
		h.Detail = json.RawMessage(detail)
		out = append(out, h)
	}
	return out, rows.Err()
}
