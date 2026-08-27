package cms

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

func ValidRateFormKey(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	for _, k := range RateFormKeys {
		if k == key {
			return true
		}
	}
	return false
}

func RateCaption(key string) string {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case RateKeyFashion:
		return "FASHION"
	case RateKeyBeauty:
		return "BEAUTY"
	case RateKeyLookbook:
		return "LOOKBOOK"
	case RateKeyEditorial:
		return "EDITORIAL"
	case RateKeyProduct:
		return "PRODUCT"
	case RateKeyManual:
		return "MANUAL"
	default:
		return strings.ToUpper(strings.TrimSpace(key))
	}
}

func RateFormValue(key string) string {
	return "rates_" + strings.ToLower(strings.TrimSpace(key))
}

func mapStringVal(data map[string]any, key string) string {
	if data == nil {
		return ""
	}
	switch v := data[key].(type) {
	case string:
		return strings.TrimSpace(v)
	case nil:
		return ""
	default:
		s := strings.TrimSpace(fmt.Sprint(v))
		if s == "<nil>" {
			return ""
		}
		return s
	}
}

// RateFormKeyFromData resolves the overlay key from form_template_id, then form_key.
func RateFormKeyFromData(data map[string]any) string {
	id := mapStringVal(data, "form_template_id")
	if key := FormKeyFromTemplateID(id); ValidRateFormKey(key) {
		return key
	}
	if ValidRateFormKey(id) {
		return strings.ToLower(strings.TrimSpace(id))
	}
	key := strings.ToLower(mapStringVal(data, "form_key"))
	if ValidRateFormKey(key) {
		return key
	}
	return ""
}

const DefaultBannerAspect = "3:4"

// BannerAspectOptions are page-level Rate grid sizes (portrait first).
var BannerAspectOptions = []string{"3:4", "2:3", "4:5", "1:1", "4:3"}

func ValidBannerAspect(s string) bool {
	s = normalizeAspectToken(s)
	for _, a := range BannerAspectOptions {
		if s == a {
			return true
		}
	}
	return false
}

func normalizeAspectToken(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "/", ":")
	s = strings.ReplaceAll(s, " ", "")
	return s
}

func NormalizeBannerAspect(s string) string {
	s = normalizeAspectToken(s)
	if ValidBannerAspect(s) {
		return s
	}
	return DefaultBannerAspect
}

func BannerAspectCSS(s string) string {
	s = NormalizeBannerAspect(s)
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return "3 / 4"
	}
	return parts[0] + " / " + parts[1]
}

func BannerAspectFromSettings(raw json.RawMessage) string {
	m := PageSettingsMap(raw)
	s, _ := m["banner_aspect"].(string)
	return NormalizeBannerAspect(s)
}

func BannerMinHeightFromSettings(raw json.RawMessage) int {
	m := PageSettingsMap(raw)
	switch v := m["banner_min_height"].(type) {
	case float64:
		return clampMinHeight(int(v))
	case int:
		return clampMinHeight(v)
	case int64:
		return clampMinHeight(int(v))
	case json.Number:
		n, _ := v.Int64()
		return clampMinHeight(int(n))
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			return 0
		}
		return clampMinHeight(n)
	default:
		return 0
	}
}

func clampMinHeight(n int) int {
	if n < 0 {
		return 0
	}
	if n > 2000 {
		return 2000
	}
	return n
}

func BannerGridStyle(raw json.RawMessage) string {
	style := "--rate-banner-aspect: " + BannerAspectCSS(BannerAspectFromSettings(raw)) + ";"
	if minH := BannerMinHeightFromSettings(raw); minH > 0 {
		style += fmt.Sprintf(" --rate-banner-min-height: %dpx;", minH)
	}
	return style
}

func RatesIntroHTML() string {
	return `<div class="rates-intro-copy">
<h2 class="rates-kicker">CHOOSE YOUR CATEGORY</h2>
<p>Click on a category to submit your retouching request.</p>
<p>The price is per photo.</p>
</div>`
}

func DefaultRateBannerData(formKey string) map[string]any {
	formKey = strings.ToLower(strings.TrimSpace(formKey))
	if !ValidRateFormKey(formKey) {
		formKey = RateKeyFashion
	}
	return map[string]any{
		"form_template_id": FormTemplateID(formKey),
		"form_key":         formKey,
		"media_id":         nil,
		"alt":              "",
		"caption":          RateCaption(formKey),
		"start_from_label": "start from",
		"price":            "",
		"currency":         "$",
		"text_color":       "", // empty = Auto (luminance at generate)
		"text_backdrop":    true, // soft white plate under overlay text
	}
}

func DefaultRatesBlocks() []map[string]any {
	out := []map[string]any{
		{"type": BlockRichText, "data": map[string]any{"html": RatesIntroHTML()}},
	}
	for _, key := range RateFormKeys {
		out = append(out, map[string]any{
			"type": BlockRateBanner,
			"data": DefaultRateBannerData(key),
		})
	}
	return out
}

func defaultRatesBlockSlice() []Block {
	raw := DefaultRatesBlocks()
	out := make([]Block, 0, len(raw))
	for _, b := range raw {
		typ, _ := b["type"].(string)
		out = append(out, Block{Type: typ, Data: MustJSON(b["data"])})
	}
	return out
}

func (s *Store) GetPageBySlug(slug string) (Page, error) {
	slug = strings.Trim(strings.TrimSpace(slug), "/")
	var id string
	err := s.db.QueryRow(`SELECT id FROM pages WHERE slug = ?`, slug).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return Page{}, ErrNotFound
	}
	if err != nil {
		return Page{}, err
	}
	return s.GetPage(id)
}

// EnsureRatesPageAndNav creates slug=rates as a draft (if missing) and places
// a visible RATES nav item immediately before ABOUT. Idempotent.
func (s *Store) EnsureRatesPageAndNav() error {
	page, err := s.GetPageBySlug("rates")
	if err != nil {
		if !errors.Is(err, ErrNotFound) {
			return err
		}
		page, err = s.CreatePage(Page{
			Slug:            "rates",
			Title:           "RATES",
			Theme:           ThemeRatesContent,
			Status:          "draft",
			SortOrder:       8,
			MetaDescription: "Retouching rates — choose a category and submit a project request.",
			Settings:        MustJSON(map[string]any{"banner_aspect": DefaultBannerAspect}),
		})
		if err != nil {
			return err
		}
		if _, err := s.ReplaceBlocks(page.ID, defaultRatesBlockSlice()); err != nil {
			return err
		}
	} else if err := s.ensureRatesIntroCopy(page); err != nil {
		return err
	}

	page, err = s.GetPageBySlug("rates")
	if err != nil {
		return err
	}
	if err := s.ensureRateBannerFormTemplates(page); err != nil {
		return err
	}
	if err := s.ensureRatesGridSettings(page); err != nil {
		return err
	}

	tree, err := s.GetNavTree()
	if err != nil {
		return err
	}
	next, changed := ensureRatesNavItem(tree, page)
	if !changed {
		return nil
	}
	_, err = s.ReplaceNav(next)
	return err
}

func looksLikeRatesIntro(html string) bool {
	return strings.Contains(html, "CHOOSE YOUR CATEGORY") &&
		strings.Contains(html, "The price is per photo")
}

var (
	reStyleAttr = regexp.MustCompile(`(?i)\sstyle="([^"]*)"`)
	reFontFam   = regexp.MustCompile(`(?i)(?:^|;)\s*font-family\s*:[^;]*`)
	reFontFace  = regexp.MustCompile(`(?i)\sface="[^"]*"`)
	reFontTag   = regexp.MustCompile(`(?i)</?font\b[^>]*>`)
)

func stripInlineFontFamily(html string) string {
	html = reFontTag.ReplaceAllString(html, "")
	html = reFontFace.ReplaceAllString(html, "")
	return reStyleAttr.ReplaceAllStringFunc(html, func(m string) string {
		sub := reStyleAttr.FindStringSubmatch(m)
		if len(sub) < 2 {
			return m
		}
		cleaned := reFontFam.ReplaceAllString(sub[1], "")
		cleaned = strings.Trim(strings.TrimSpace(strings.ReplaceAll(cleaned, ";;", ";")), "; ")
		if cleaned == "" {
			return ""
		}
		return ` style="` + cleaned + `"`
	})
}

func (s *Store) ensureRatesIntroCopy(page Page) error {
	changed := false
	blocks := append([]Block(nil), page.Blocks...)
	for i, b := range blocks {
		if b.Type != BlockRichText {
			continue
		}
		m := PageSettingsMap(b.Data)
		html, _ := m["html"].(string)
		want := html
		if looksLikeRatesIntro(html) {
			want = RatesIntroHTML()
		} else {
			want = stripInlineFontFamily(html)
		}
		if html == want {
			continue
		}
		m["html"] = want
		blocks[i].Data = MustJSON(m)
		changed = true
	}
	if !changed {
		return nil
	}
	_, err := s.ReplaceBlocks(page.ID, blocks)
	return err
}

func (s *Store) ensureRateBannerFormTemplates(page Page) error {
	fresh, err := s.GetPage(page.ID)
	if err != nil {
		return err
	}
	changed := false
	blocks := append([]Block(nil), fresh.Blocks...)
	for i, b := range blocks {
		if b.Type != BlockRateBanner {
			continue
		}
		m := PageSettingsMap(b.Data)
		key := RateFormKeyFromData(m)
		if !ValidRateFormKey(key) {
			continue
		}
		wantID := FormTemplateID(key)
		curID := mapStringVal(m, "form_template_id")
		curKey := strings.ToLower(mapStringVal(m, "form_key"))
		if curID == wantID && curKey == key {
			continue
		}
		m["form_template_id"] = wantID
		m["form_key"] = key
		blocks[i].Data = MustJSON(m)
		changed = true
	}
	if !changed {
		return nil
	}
	_, err = s.ReplaceBlocks(fresh.ID, blocks)
	return err
}

func (s *Store) ensureRatesGridSettings(page Page) error {
	fresh, err := s.GetPage(page.ID)
	if err != nil {
		return err
	}
	m := PageSettingsMap(fresh.Settings)
	raw, _ := m["banner_aspect"].(string)
	if ValidBannerAspect(raw) {
		return nil
	}
	return s.MergePageSettings(fresh.ID, map[string]any{"banner_aspect": DefaultBannerAspect})
}

func ensureRatesNavItem(tree []NavItem, page Page) ([]NavItem, bool) {
	stripped, existing := stripRatesNav(tree)
	if stripped == nil {
		stripped = []NavItem{}
	}
	item := NavItem{
		Label:    "RATES",
		Href:     HrefForPage(page),
		PageID:   page.ID,
		Kind:     NavKindLink,
		Visible:  true,
		Children: []NavItem{},
	}
	if existing != nil {
		item.ID = existing.ID
		if strings.TrimSpace(existing.Label) != "" {
			item.Label = existing.Label
		}
	}

	aboutIdx := indexNavLabelOrHref(stripped, "ABOUT", "about")
	wantIdx := aboutIdx
	if wantIdx < 0 {
		wantIdx = indexNavLabelOrHref(stripped, "CONTACT", "contact")
	}
	if wantIdx < 0 {
		wantIdx = len(stripped)
	}

	already := false
	if existing != nil && existing.ParentID == "" {
		curIdx := indexNavID(tree, existing.ID)
		if curIdx == wantIdx && existing.Visible && existing.PageID == page.ID {
			href := strings.Trim(existing.Href, "/")
			already = href == "rates"
		}
	}
	if already {
		return tree, false
	}

	out := make([]NavItem, 0, len(stripped)+1)
	out = append(out, stripped[:wantIdx]...)
	out = append(out, item)
	out = append(out, stripped[wantIdx:]...)
	return out, true
}

func stripRatesNav(items []NavItem) ([]NavItem, *NavItem) {
	out := make([]NavItem, 0, len(items))
	var found *NavItem
	for _, it := range items {
		if it.Kind != NavKindCategory && isRatesNavItem(it) {
			cp := it
			found = &cp
			continue
		}
		kids, kidFound := stripRatesNav(it.Children)
		it.Children = kids
		if kidFound != nil && found == nil {
			found = kidFound
		}
		out = append(out, it)
	}
	return out, found
}

func isRatesNavItem(it NavItem) bool {
	if strings.EqualFold(strings.TrimSpace(it.Label), "RATES") {
		return true
	}
	return strings.Trim(it.Href, "/") == "rates"
}

func indexNavLabelOrHref(items []NavItem, label, slug string) int {
	for i, it := range items {
		if strings.EqualFold(strings.TrimSpace(it.Label), label) {
			return i
		}
		if strings.Trim(it.Href, "/") == slug {
			return i
		}
	}
	return -1
}

func indexNavID(items []NavItem, id string) int {
	if id == "" {
		return -1
	}
	for i, it := range items {
		if it.ID == id {
			return i
		}
	}
	return -1
}
