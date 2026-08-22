package cms

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

var ErrNotFound = errors.New("not found")
var ErrInvalidNav = errors.New("invalid nav")

func (s *Store) GetSettings() (SiteSettings, error) {
	var st SiteSettings
	err := s.db.QueryRow(`
SELECT site_name, logo_html, description, og_image, instagram_url, behance_url,
       linkedin_url, copyright, canonical_base, contact_email, updated_at
FROM site_settings WHERE id = 1`).Scan(
		&st.SiteName, &st.LogoHTML, &st.Description, &st.OGImage,
		&st.InstagramURL, &st.BehanceURL, &st.LinkedInURL, &st.Copyright,
		&st.CanonicalBase, &st.ContactEmail, &st.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return SiteSettings{}, nil
	}
	if err != nil {
		return SiteSettings{}, err
	}
	st.MailtoAddress = st.ContactEmail
	return st, nil
}

func (s *Store) PutSettings(st SiteSettings) (SiteSettings, error) {
	st.ContactEmail = strings.TrimSpace(st.ContactEmail)
	if st.ContactEmail == "" {
		st.ContactEmail = strings.TrimSpace(st.MailtoAddress)
	}
	st.UpdatedAt = Now()
	_, err := s.db.Exec(`
INSERT INTO site_settings (
  id, site_name, logo_html, description, og_image, instagram_url, behance_url,
  linkedin_url, copyright, canonical_base, contact_email, updated_at
) VALUES (1,?,?,?,?,?,?,?,?,?,?,?)
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
  contact_email=excluded.contact_email,
  updated_at=excluded.updated_at
`, st.SiteName, st.LogoHTML, st.Description, st.OGImage, st.InstagramURL,
		st.BehanceURL, st.LinkedInURL, st.Copyright, st.CanonicalBase, st.ContactEmail, st.UpdatedAt)
	if err != nil {
		return SiteSettings{}, err
	}
	return s.GetSettings()
}

// ContactRecipient is the inbox for public contact-form mail: settings first,
// then a mailto saved on a contact_form block (admin page inspector).
func (s *Store) ContactRecipient() (string, error) {
	st, err := s.GetSettings()
	if err != nil {
		return "", err
	}
	if e := strings.TrimSpace(st.ContactEmail); e != "" {
		return e, nil
	}
	return s.firstContactMailto()
}

// BackfillContactEmail copies a contact_form block mailto into settings when
// contact_email is still empty (one-time after adding the column).
func (s *Store) BackfillContactEmail() error {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM site_settings WHERE id = 1`).Scan(&n)
	if err != nil {
		return err
	}
	if n == 0 {
		return nil
	}
	st, err := s.GetSettings()
	if err != nil {
		return err
	}
	if strings.TrimSpace(st.ContactEmail) != "" {
		return nil
	}
	mailto, err := s.firstContactMailto()
	if err != nil || mailto == "" {
		return err
	}
	_, err = s.db.Exec(`UPDATE site_settings SET contact_email = ?, updated_at = ? WHERE id = 1`, mailto, Now())
	return err
}

func (s *Store) firstContactMailto() (string, error) {
	rows, err := s.db.Query(`SELECT data_json FROM blocks WHERE type = ?`, BlockContactForm)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return "", err
		}
		var data struct {
			Mailto string `json:"mailto"`
		}
		if err := json.Unmarshal([]byte(raw), &data); err != nil {
			continue
		}
		if e := strings.TrimSpace(data.Mailto); e != "" {
			return e, nil
		}
	}
	return "", rows.Err()
}

const pageSelectCols = `id, slug, title, theme, status, sort_order, meta_title, meta_description,
       canonical_path, og_image, is_homepage, settings_json, created_at, updated_at`

func (s *Store) ListPages() ([]Page, error) {
	rows, err := s.db.Query(`
SELECT ` + pageSelectCols + `
FROM pages ORDER BY sort_order ASC, title ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Page, 0)
	for rows.Next() {
		p, err := scanPage(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) GetPage(id string) (Page, error) {
	p, err := scanPage(s.db.QueryRow(`
SELECT `+pageSelectCols+`
FROM pages WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Page{}, ErrNotFound
	}
	if err != nil {
		return Page{}, err
	}
	blocks, err := s.ListBlocks(id)
	if err != nil {
		return Page{}, err
	}
	p.Blocks = blocks
	p.NormalizeAliases()
	return p, nil
}

func scanPage(scanner interface {
	Scan(dest ...any) error
}) (Page, error) {
	var p Page
	var home int
	var settings string
	if err := scanner.Scan(
		&p.ID, &p.Slug, &p.Title, &p.Theme, &p.Status, &p.SortOrder,
		&p.MetaTitle, &p.MetaDescription, &p.CanonicalPath, &p.OGImage, &home, &settings,
		&p.CreatedAt, &p.UpdatedAt,
	); err != nil {
		return Page{}, err
	}
	p.IsHomepage = home == 1
	p.Settings = json.RawMessage(settings)
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
	if len(p.Settings) == 0 || string(p.Settings) == "null" {
		p.Settings = json.RawMessage(`{}`)
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
INSERT INTO pages (id, slug, title, theme, status, sort_order, meta_title, meta_description, canonical_path, og_image, is_homepage, settings_json, created_at, updated_at)
VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		p.ID, p.Slug, p.Title, p.Theme, p.Status, p.SortOrder, p.MetaTitle, p.MetaDescription, p.CanonicalPath, p.OGImage, home, string(p.Settings), p.CreatedAt, p.UpdatedAt)
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
	FlattenSEOPatch(patch)
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
	if v, ok := patch["meta_title"].(string); ok {
		cur.MetaTitle = v
	}
	if v, ok := patch["meta_description"].(string); ok {
		cur.MetaDescription = v
	}
	if v, ok := patch["canonical_path"].(string); ok {
		cur.CanonicalPath = v
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
	if raw, ok := patch["settings"]; ok {
		cur.Settings = MergeSettings(cur.Settings, raw)
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
UPDATE pages SET slug=?, title=?, theme=?, status=?, sort_order=?, meta_title=?, meta_description=?,
  canonical_path=?, og_image=?, is_homepage=?, settings_json=?, updated_at=? WHERE id=?`,
		cur.Slug, cur.Title, cur.Theme, cur.Status, cur.SortOrder, cur.MetaTitle, cur.MetaDescription,
		cur.CanonicalPath, cur.OGImage, home, string(cur.Settings), cur.UpdatedAt, id)
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

// MergePageSettings overlays keys into pages.settings_json without touching nav.
func (s *Store) MergePageSettings(id string, patch map[string]any) error {
	cur, err := s.GetPage(id)
	if err != nil {
		return err
	}
	raw := MergeSettings(cur.Settings, patch)
	_, err = s.db.Exec(`UPDATE pages SET settings_json=?, updated_at=? WHERE id=?`, string(raw), Now(), id)
	return err
}

// SyncNavForPage updates menu label/href for all nav rows linked to this page.
func (s *Store) SyncNavForPage(p Page) error {
	label := strings.TrimSpace(p.NavLabel)
	if label == "" {
		label = strings.TrimSpace(p.Title)
	}
	href := HrefForPage(p)
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
FROM nav ORDER BY parent_id ASC, sort_order ASC, label ASC`)
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
		n.Children = []NavItem{}
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
		if item.Kind != NavKindCategory && item.PageID != "" {
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
		if r.Children == nil {
			r.Children = []NavItem{}
		}
		out = append(out, *r)
	}
	return out, nil
}

func (s *Store) ReplaceNav(items []NavItem) ([]NavItem, error) {
	prepared, err := s.prepareNavTree(items)
	if err != nil {
		return nil, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`DELETE FROM nav`); err != nil {
		return nil, err
	}
	var insert func(parentID string, list []NavItem) error
	insert = func(parentID string, list []NavItem) error {
		for i, item := range list {
			if item.ID == "" {
				item.ID = NewID()
			}
			vis := 0
			if item.Visible {
				vis = 1
			}
			if _, err := tx.Exec(`
INSERT INTO nav (id, label, href, page_id, parent_id, sort_order, kind, visible)
VALUES (?,?,?,?,?,?,?,?)`,
				item.ID, item.Label, item.Href, item.PageID, parentID, i, item.Kind, vis); err != nil {
				return err
			}
			if err := insert(item.ID, item.Children); err != nil {
				return err
			}
		}
		return nil
	}
	if err := insert("", prepared); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetNavTree()
}

func (s *Store) prepareNavTree(items []NavItem) ([]NavItem, error) {
	pages, err := s.ListPages()
	if err != nil {
		return nil, err
	}
	byID := make(map[string]Page, len(pages))
	for _, p := range pages {
		byID[p.ID] = p
	}
	seen := map[string]struct{}{}
	var walk func(list []NavItem, parentKind string) ([]NavItem, error)
	walk = func(list []NavItem, parentKind string) ([]NavItem, error) {
		out := make([]NavItem, 0, len(list))
		for i, item := range list {
			item.Label = strings.TrimSpace(item.Label)
			if item.Label == "" {
				return nil, fmt.Errorf("%w: label required", ErrInvalidNav)
			}
			item.Href = strings.TrimSpace(item.Href)
			item.PageID = strings.TrimSpace(item.PageID)
			item.Kind = strings.ToLower(strings.TrimSpace(item.Kind))
			if item.Kind == "" {
				if len(item.Children) > 0 {
					item.Kind = NavKindCategory
				} else {
					item.Kind = NavKindLink
				}
			}
			if item.Kind != NavKindLink && item.Kind != NavKindCategory {
				return nil, fmt.Errorf("%w: kind must be %q or %q", ErrInvalidNav, NavKindLink, NavKindCategory)
			}
			if parentKind == NavKindCategory && item.Kind != NavKindLink {
				return nil, fmt.Errorf("%w: dropdown children must be kind=link (one-level menus)", ErrInvalidNav)
			}
			if item.Kind == NavKindLink && len(item.Children) > 0 {
				return nil, fmt.Errorf("%w: kind=link cannot have children", ErrInvalidNav)
			}
			if item.ID != "" {
				if _, ok := seen[item.ID]; ok {
					return nil, fmt.Errorf("%w: duplicate id %q", ErrInvalidNav, item.ID)
				}
				seen[item.ID] = struct{}{}
			}
			if item.Kind == NavKindLink {
				if item.PageID != "" {
					p, ok := byID[item.PageID]
					if !ok {
						return nil, fmt.Errorf("%w: unknown page_id %q", ErrInvalidNav, item.PageID)
					}
					if item.Href == "" {
						item.Href = HrefForPage(p)
					}
				}
			}
			kids, err := walk(item.Children, item.Kind)
			if err != nil {
				return nil, err
			}
			if kids == nil {
				kids = []NavItem{}
			}
			item.Children = kids
			item.ParentID = ""
			item.SortOrder = i
			out = append(out, item)
		}
		return out, nil
	}
	return walk(items, "")
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
