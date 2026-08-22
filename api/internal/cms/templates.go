package cms

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const templateSelectCols = `id, theme, name, description, allowed_blocks_json, default_blocks_json,
       is_system, sort_order, kind, form_key, source, created_at, updated_at`

func scanTemplate(scanner interface {
	Scan(dest ...any) error
}) (Template, error) {
	var t Template
	var allowedRaw, defaultRaw string
	var system int
	if err := scanner.Scan(
		&t.ID, &t.Theme, &t.Name, &t.Description,
		&allowedRaw, &defaultRaw, &system, &t.SortOrder,
		&t.Kind, &t.FormKey, &t.Source, &t.CreatedAt, &t.UpdatedAt,
	); err != nil {
		return Template{}, err
	}
	t.IsSystem = system == 1
	if err := json.Unmarshal([]byte(allowedRaw), &t.AllowedBlocks); err != nil {
		t.AllowedBlocks = []string{}
	}
	t.DefaultBlocks = json.RawMessage(defaultRaw)
	t.NormalizeAliases()
	return t, nil
}

func (s *Store) ListTemplates() ([]Template, error) {
	rows, err := s.db.Query(`
SELECT ` + templateSelectCols + `
FROM templates ORDER BY sort_order ASC, name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Template, 0)
	for rows.Next() {
		t, err := scanTemplate(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) GetTemplate(id string) (Template, error) {
	row := s.db.QueryRow(`
SELECT `+templateSelectCols+`
FROM templates WHERE id = ?`, id)
	t, err := scanTemplate(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Template{}, ErrNotFound
	}
	return t, err
}

func (s *Store) CreateTemplate(t Template) (Template, error) {
	t.NormalizeAliases()
	if strings.TrimSpace(t.Name) == "" {
		return Template{}, fmt.Errorf("name required")
	}
	if t.Kind != TemplateKindPage && t.Kind != TemplateKindForm {
		return Template{}, fmt.Errorf("invalid kind: must be page or form")
	}
	if t.Kind == TemplateKindForm {
		if t.Theme == "" {
			t.Theme = ThemeRatesContent
		}
		if t.FormKey == "" {
			t.FormKey = FormKeyFromTemplateID(t.ID)
		}
		t.FormKey = strings.ToLower(strings.TrimSpace(t.FormKey))
		if !ValidRateFormKey(t.FormKey) {
			return Template{}, fmt.Errorf("form_key must be a rates form (fashion, beauty, lookbook, editorial, product, or manual)")
		}
		if t.ID == "" {
			t.ID = FormTemplateID(t.FormKey)
		}
	} else {
		t.FormKey = ""
		if t.Theme == "" {
			return Template{}, fmt.Errorf("theme required")
		}
	}
	if !ValidTheme(t.Theme) {
		return Template{}, fmt.Errorf("invalid theme: must be %s", ValidThemeList())
	}
	if t.Kind == TemplateKindForm {
		t.AllowedBlocks = []string{}
		if IsEmptyJSONArray(t.DefaultBlocks) {
			t.DefaultBlocks = DefaultFormBlocks(t.FormKey)
		}
	} else if err := validateAllowedBlocks(t.AllowedBlocks); err != nil {
		return Template{}, err
	}
	if err := validateDefaultBlocksJSON(t.DefaultBlocks, t.Kind); err != nil {
		return Template{}, err
	}
	if t.Kind != TemplateKindForm && len(t.AllowedBlocks) == 0 {
		t.AllowedBlocks = DefaultAllowedBlocks(t.Theme)
	}
	if t.ID == "" {
		t.ID = NewID()
	}
	t.ID = strings.TrimSpace(t.ID)
	if t.ID == "" {
		return Template{}, fmt.Errorf("id required")
	}
	// Custom templates cannot reuse system ids unless creating that system row.
	if !t.IsSystem && IsReservedTemplateID(t.ID) {
		return Template{}, fmt.Errorf("id %q is reserved for system templates", t.ID)
	}
	now := Now()
	t.CreatedAt = now
	t.UpdatedAt = now
	sys := 0
	if t.IsSystem {
		sys = 1
	}
	allowedJSON, _ := json.Marshal(t.AllowedBlocks)
	if len(t.DefaultBlocks) == 0 {
		t.DefaultBlocks = json.RawMessage(`[]`)
	}
	_, err := s.db.Exec(`
INSERT INTO templates (
  id, theme, name, description, allowed_blocks_json, default_blocks_json,
  is_system, sort_order, kind, form_key, source, created_at, updated_at
) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		t.ID, t.Theme, t.Name, t.Description, string(allowedJSON), string(t.DefaultBlocks),
		sys, t.SortOrder, t.Kind, t.FormKey, t.Source, t.CreatedAt, t.UpdatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "constraint") {
			return Template{}, fmt.Errorf("template id already exists")
		}
		return Template{}, err
	}
	return s.GetTemplate(t.ID)
}

func (s *Store) PatchTemplate(id string, patch map[string]any) (Template, error) {
	cur, err := s.GetTemplate(id)
	if err != nil {
		return Template{}, err
	}

	if v, ok := patch["name"].(string); ok {
		cur.Name = v
	}
	if v, ok := patch["label"].(string); ok {
		cur.Name = v
	}
	if v, ok := patch["description"].(string); ok {
		cur.Description = v
	}
	if v, ok := patch["sort_order"].(float64); ok {
		cur.SortOrder = int(v)
	}

	// Theme/engine: only custom templates may change it; system keeps fixed engine.
	if !cur.IsSystem {
		if v, ok := patch["theme"].(string); ok {
			if !ValidTheme(v) {
				return Template{}, fmt.Errorf("invalid theme")
			}
			cur.Theme = v
		}
		if v, ok := patch["key"].(string); ok {
			if !ValidTheme(v) {
				return Template{}, fmt.Errorf("invalid theme")
			}
			cur.Theme = v
		}
	}

	if raw, ok := patch["allowed_blocks"]; ok && cur.Kind != TemplateKindForm {
		blocks, err := coerceStringSlice(raw)
		if err != nil {
			return Template{}, fmt.Errorf("allowed_blocks: %w", err)
		}
		if err := validateAllowedBlocks(blocks); err != nil {
			return Template{}, err
		}
		cur.AllowedBlocks = blocks
	}
	if raw, ok := patch["default_blocks"]; ok {
		b, err := json.Marshal(raw)
		if err != nil {
			return Template{}, fmt.Errorf("default_blocks: bad json")
		}
		if err := validateDefaultBlocksJSON(b, cur.Kind); err != nil {
			return Template{}, err
		}
		cur.DefaultBlocks = b
	} else if raw, ok := patch["fields"]; ok && cur.Kind == TemplateKindForm {
		b, err := json.Marshal(raw)
		if err != nil {
			return Template{}, fmt.Errorf("fields: bad json")
		}
		if err := validateDefaultBlocksJSON(b, cur.Kind); err != nil {
			return Template{}, err
		}
		cur.DefaultBlocks = b
	}
	if v, ok := patch["source"].(string); ok && cur.Kind != TemplateKindForm {
		cur.Source = v
	} else if v, ok := patch["body"].(string); ok && cur.Kind != TemplateKindForm {
		cur.Source = v
	}

	if strings.TrimSpace(cur.Name) == "" {
		return Template{}, fmt.Errorf("name required")
	}
	if !cur.IsSystem {
		if v, ok := patch["kind"].(string); ok {
			k := strings.ToLower(strings.TrimSpace(v))
			if k != TemplateKindPage && k != TemplateKindForm {
				return Template{}, fmt.Errorf("invalid kind: must be page or form")
			}
			cur.Kind = k
		}
		if v, ok := patch["form_key"].(string); ok {
			cur.FormKey = strings.ToLower(strings.TrimSpace(v))
		}
	}
	if cur.Kind == TemplateKindForm {
		if cur.FormKey == "" {
			cur.FormKey = FormKeyFromTemplateID(cur.ID)
		}
		if !ValidRateFormKey(cur.FormKey) {
			return Template{}, fmt.Errorf("form_key must be a rates form")
		}
	} else {
		cur.FormKey = ""
	}

	if cur.Kind == TemplateKindForm {
		cur.Source = ""
		cur.AllowedBlocks = []string{}
	}
	cur.UpdatedAt = Now()
	allowedJSON, _ := json.Marshal(cur.AllowedBlocks)
	_, err = s.db.Exec(`
UPDATE templates SET theme=?, name=?, description=?, allowed_blocks_json=?,
  default_blocks_json=?, sort_order=?, kind=?, form_key=?, source=?, updated_at=? WHERE id=?`,
		cur.Theme, cur.Name, cur.Description, string(allowedJSON),
		string(cur.DefaultBlocks), cur.SortOrder, cur.Kind, cur.FormKey, cur.Source, cur.UpdatedAt, id)
	if err != nil {
		return Template{}, err
	}
	return s.GetTemplate(id)
}

func (s *Store) PutTemplate(id string, t Template) (Template, error) {
	if _, err := s.GetTemplate(id); err != nil {
		return Template{}, err
	}
	t.NormalizeAliases()
	if strings.TrimSpace(t.Name) == "" {
		return Template{}, fmt.Errorf("name required")
	}
	patch := map[string]any{
		"name":           t.Name,
		"description":    t.Description,
		"sort_order":     float64(t.SortOrder),
		"allowed_blocks": t.AllowedBlocks,
		"source":         t.Source,
	}
	var raw any
	if err := json.Unmarshal(t.DefaultBlocks, &raw); err != nil || raw == nil {
		raw = []any{}
	}
	patch["default_blocks"] = raw
	if t.Theme != "" {
		patch["theme"] = t.Theme
	}
	return s.PatchTemplate(id, patch)
}

func coerceStringSlice(v any) ([]string, error) {
	switch x := v.(type) {
	case []string:
		return x, nil
	case []any:
		out := make([]string, 0, len(x))
		for _, item := range x {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("must be string array")
			}
			out = append(out, s)
		}
		return out, nil
	case nil:
		return []string{}, nil
	default:
		return nil, fmt.Errorf("must be string array")
	}
}

func validateAllowedBlocks(blocks []string) error {
	for _, b := range blocks {
		if !ValidBlockType(b) {
			return fmt.Errorf("invalid block type %q", b)
		}
	}
	return nil
}

func validateDefaultBlocksJSON(raw json.RawMessage, kind string) error {
	if len(raw) == 0 {
		return nil
	}
	var blocks []struct {
		Type string          `json:"type"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return fmt.Errorf("default_blocks must be an array")
	}
	for _, b := range blocks {
		if !ValidTemplateBlockType(kind, b.Type) {
			return fmt.Errorf("invalid default block type %q", b.Type)
		}
	}
	return nil
}

// EnsureSystemTemplates inserts built-in templates when missing (does not overwrite edits).
// Empty form canvases are seeded once from the current Beauty/Fashion/… field sets.
func (s *Store) EnsureSystemTemplates() error {
	for _, t := range builtinSystemTemplates() {
		cur, err := s.GetTemplate(t.ID)
		if errors.Is(err, ErrNotFound) {
			if _, err := s.CreateTemplate(t); err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		if cur.Kind == TemplateKindForm && IsEmptyJSONArray(cur.DefaultBlocks) {
			seed := DefaultFormBlocks(cur.FormKey)
			if _, err := s.PatchTemplate(cur.ID, map[string]any{
				"default_blocks": jsonArrayAny(seed),
			}); err != nil {
				return err
			}
		}
	}
	return s.ensureArticleImageAllowedBlocks()
}

func jsonArrayAny(raw json.RawMessage) any {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil || v == nil {
		return []any{}
	}
	return v
}

func builtinSystemTemplates() []Template {
	sliderData := map[string]any{"before_media_id": nil, "after_media_id": nil, "caption": ""}
	galleryData := map[string]any{"media_id": nil, "alt": "", "caption": ""}
	return []Template{
		{
			ID:            ThemeBAContent,
			Theme:         ThemeBAContent,
			Name:          "Before | After",
			Description:   "Stacked comparison sliders (home / before-after).",
			AllowedBlocks: []string{BlockComparisonSlider},
			DefaultBlocks: MustJSON([]map[string]any{
				{"type": BlockComparisonSlider, "data": sliderData},
				{"type": BlockComparisonSlider, "data": sliderData},
			}),
			IsSystem:  true,
			SortOrder: 0,
		},
		{
			ID:            ThemePanoramaGallery,
			Theme:         ThemePanoramaGallery,
			Name:          "Gallery",
			Description:   "Vertical panorama image strip (editorial / fashion).",
			AllowedBlocks: []string{BlockGalleryImage},
			DefaultBlocks: MustJSON([]map[string]any{
				{"type": BlockGalleryImage, "data": galleryData},
				{"type": BlockGalleryImage, "data": galleryData},
				{"type": BlockGalleryImage, "data": galleryData},
			}),
			IsSystem:  true,
			SortOrder: 1,
		},
		{
			ID:            ThemeTextContent,
			Theme:         ThemeTextContent,
			Name:          "Text / blank",
			Description:   "Rich text, images, and optional contact form.",
			AllowedBlocks: []string{BlockRichText, BlockGalleryImage, BlockContactForm},
			DefaultBlocks: MustJSON([]map[string]any{
				{"type": BlockRichText, "data": map[string]any{"html": "<p></p>"}},
			}),
			IsSystem:  true,
			SortOrder: 2,
		},
		{
			ID:            ThemeAboutContent,
			Theme:         ThemeAboutContent,
			Name:          "About",
			Description:   "Portrait on the left, bio on the right — centered with wide margins.",
			AllowedBlocks: []string{BlockGalleryImage, BlockRichText},
			DefaultBlocks: MustJSON(DefaultAboutBlocks()),
			IsSystem:      true,
			SortOrder:     2,
		},
		{
			ID:            ThemeLookbookGallery,
			Theme:         ThemeLookbookGallery,
			Name:          "Lookbook",
			Description:   "Masonry photo grid; click opens the panorama overlay.",
			AllowedBlocks: []string{BlockGalleryImage},
			DefaultBlocks: MustJSON([]map[string]any{
				{"type": BlockGalleryImage, "data": galleryData},
				{"type": BlockGalleryImage, "data": galleryData},
				{"type": BlockGalleryImage, "data": galleryData},
				{"type": BlockGalleryImage, "data": galleryData},
			}),
			IsSystem:  true,
			SortOrder: 3,
		},
		{
			ID:            ThemeRatesContent,
			Theme:         ThemeRatesContent,
			Kind:          TemplateKindPage,
			Name:          "Rates",
			Description:   "Category banners; each Rate banner chooses a form template (Fashion, Beauty, …).",
			AllowedBlocks: []string{BlockRichText, BlockRateBanner},
			DefaultBlocks: MustJSON(DefaultRatesBlocks()),
			IsSystem:      true,
			SortOrder:     4,
		},
		builtinFormTemplate(FormTemplateFashion, "Fashion", RateKeyFashion, 10),
		builtinFormTemplate(FormTemplateBeauty, "Beauty", RateKeyBeauty, 11),
		builtinFormTemplate(FormTemplateLookbook, "Lookbook", RateKeyLookbook, 12),
		builtinFormTemplate(FormTemplateEditorial, "Editorial", RateKeyEditorial, 13),
		builtinFormTemplate(FormTemplateProduct, "Product", RateKeyProduct, 14),
		builtinFormTemplate(FormTemplateManual, "Manual", RateKeyManual, 15),
	}
}

func builtinFormTemplate(id, name, formKey string, sort int) Template {
	return Template{
		ID:            id,
		Theme:         ThemeRatesContent,
		Kind:          TemplateKindForm,
		FormKey:       formKey,
		Name:          name,
		Description:   name + " project request form. Connect this template on a Rate banner.",
		AllowedBlocks: []string{},
		DefaultBlocks: DefaultFormBlocks(formKey),
		IsSystem:      true,
		SortOrder:     sort,
	}
}
