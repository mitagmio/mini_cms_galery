package cms

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"
)

func NewID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func Now() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func MustJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return b
}

func ValidTheme(t string) bool {
	switch t {
	case ThemeBAContent, ThemePanoramaGallery, ThemeTextContent, ThemeLookbookGallery:
		return true
	default:
		return false
	}
}

// IsReservedTemplateID is true for built-in system template ids (id == theme key).
func IsReservedTemplateID(id string) bool {
	return ValidTheme(id)
}

// ValidThemeList is the human-readable set of generate engines.
func ValidThemeList() string {
	return "ba_content, panorama_gallery, text_content, or lookbook_gallery"
}

// HrefForPage is the site-root path for a CMS page (homepage → "/").
func HrefForPage(p Page) string {
	if p.IsHomepage || strings.TrimSpace(p.Slug) == "" {
		return "/"
	}
	return "/" + strings.Trim(p.Slug, "/")
}

func ValidBlockType(t string) bool {
	switch t {
	case BlockComparisonSlider, BlockGalleryImage, BlockRichText, BlockContactForm:
		return true
	default:
		return false
	}
}

// DefaultAllowedBlocks returns the block types that fit a theme engine.
func DefaultAllowedBlocks(theme string) []string {
	switch theme {
	case ThemeBAContent:
		return []string{BlockComparisonSlider}
	case ThemePanoramaGallery, ThemeLookbookGallery:
		return []string{BlockGalleryImage}
	case ThemeTextContent:
		return []string{BlockRichText, BlockContactForm}
	default:
		return []string{}
	}
}

// PageSettingsMap decodes pages.settings_json (object or empty).
func PageSettingsMap(raw json.RawMessage) map[string]any {
	out := map[string]any{}
	if len(raw) == 0 || string(raw) == "null" {
		return out
	}
	_ = json.Unmarshal(raw, &out)
	if out == nil {
		return map[string]any{}
	}
	return out
}

// ShuffleSeed reads settings.shuffle_seed (JSON number).
func ShuffleSeed(raw json.RawMessage) int64 {
	m := PageSettingsMap(raw)
	switch v := m["shuffle_seed"].(type) {
	case float64:
		return int64(v)
	case int64:
		return v
	case json.Number:
		n, _ := v.Int64()
		return n
	default:
		return 0
	}
}

// MergeSettings overlays keys from patch onto base settings JSON.
func MergeSettings(base json.RawMessage, patch any) json.RawMessage {
	m := PageSettingsMap(base)
	var extra map[string]any
	switch p := patch.(type) {
	case map[string]any:
		extra = p
	case json.RawMessage:
		extra = PageSettingsMap(p)
	default:
		b, err := json.Marshal(p)
		if err != nil {
			return MustJSON(m)
		}
		extra = PageSettingsMap(b)
	}
	for k, v := range extra {
		m[k] = v
	}
	return MustJSON(m)
}
