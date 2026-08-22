package cms

import (
	"encoding/json"
	"errors"
	"strings"
)

func DefaultAboutBlocks() []map[string]any {
	return []map[string]any{
		{"type": BlockGalleryImage, "data": map[string]any{"media_id": nil, "alt": "", "caption": ""}},
		{"type": BlockRichText, "data": map[string]any{"html": "<p></p>"}},
	}
}

func defaultAboutBlockSlice() []Block {
	raw := DefaultAboutBlocks()
	out := make([]Block, 0, len(raw))
	for _, b := range raw {
		typ, _ := b["type"].(string)
		out = append(out, Block{Type: typ, Data: MustJSON(b["data"])})
	}
	return out
}

func isTextOrBlankTemplate(t Template) bool {
	if t.Kind == TemplateKindForm {
		return false
	}
	if t.Theme == ThemeTextContent || t.ID == ThemeTextContent {
		return true
	}
	n := strings.ToLower(strings.TrimSpace(t.Name + " " + t.ID + " " + t.Label))
	return strings.Contains(n, "blank")
}

func insertAfter(list []string, after, add string) []string {
	if containsString(list, add) {
		return list
	}
	out := make([]string, 0, len(list)+1)
	placed := false
	for _, s := range list {
		out = append(out, s)
		if s == after {
			out = append(out, add)
			placed = true
		}
	}
	if !placed {
		out = append(out, add)
	}
	return out
}

func migratePageTemplateAllowed(t Template) ([]string, bool) {
	if t.Kind == TemplateKindForm {
		return t.AllowedBlocks, false
	}
	cur := append([]string{}, t.AllowedBlocks...)
	switch {
	case t.ID == ThemeAboutContent || t.Theme == ThemeAboutContent:
		if len(cur) == 0 {
			return DefaultAllowedBlocks(ThemeAboutContent), true
		}
		next := withString(cur, BlockGalleryImage)
		next = withString(next, BlockRichText)
		return next, !stringsEqual(cur, next)
	case isTextOrBlankTemplate(t):
		if len(cur) == 0 {
			return DefaultAllowedBlocks(ThemeTextContent), true
		}
		next := insertAfter(cur, BlockRichText, BlockGalleryImage)
		return next, !stringsEqual(cur, next)
	default:
		return cur, false
	}
}

// ensureArticleImageAllowedBlocks adds gallery_image to text/blank templates
// and keeps About allowed blocks as portrait + bio.
func (s *Store) ensureArticleImageAllowedBlocks() error {
	list, err := s.ListTemplates()
	if err != nil {
		return err
	}
	for _, t := range list {
		next, changed := migratePageTemplateAllowed(t)
		if !changed {
			continue
		}
		if _, err := s.PatchTemplate(t.ID, map[string]any{"allowed_blocks": next}); err != nil {
			return err
		}
	}
	return nil
}

func aboutThemeSwitchable(theme string) bool {
	switch theme {
	case ThemeTextContent, ThemePanoramaGallery, ThemeLookbookGallery, "":
		return true
	default:
		return false
	}
}

func aboutBlocksAreEmpty(blocks []Block) bool {
	if len(blocks) == 0 {
		return true
	}
	for _, b := range blocks {
		var data map[string]any
		_ = json.Unmarshal(b.Data, &data)
		switch b.Type {
		case BlockRichText:
			html, _ := data["html"].(string)
			if richTextHasCopy(html) {
				return false
			}
		case BlockGalleryImage:
			if galleryDataHasMedia(data) {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func richTextHasCopy(html string) bool {
	s := strings.ToLower(strings.TrimSpace(html))
	if s == "" || s == "<p></p>" || s == "<p><br></p>" || s == "<p><br/></p>" {
		return false
	}
	repl := []string{"<p>", "</p>", "<br>", "<br/>", "<br />", "&nbsp;", " "}
	for _, r := range repl {
		s = strings.ReplaceAll(s, r, "")
	}
	return strings.TrimSpace(s) != ""
}

func galleryDataHasMedia(data map[string]any) bool {
	if data == nil {
		return false
	}
	if id, _ := data["media_id"].(string); strings.TrimSpace(id) != "" && strings.TrimSpace(id) != "<nil>" {
		return true
	}
	if url, _ := data["url"].(string); strings.TrimSpace(url) != "" {
		return true
	}
	return false
}

// EnsureAboutPage switches slug=about onto about_content without wiping copy.
// Empty canvases get the portrait + bio starters.
func (s *Store) EnsureAboutPage() error {
	page, err := s.GetPageBySlug("about")
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if page.Theme != ThemeAboutContent && aboutThemeSwitchable(page.Theme) {
		page, err = s.PatchPage(page.ID, map[string]any{"theme": ThemeAboutContent})
		if err != nil {
			return err
		}
	}
	if page.Theme == ThemeAboutContent && aboutBlocksAreEmpty(page.Blocks) {
		if _, err := s.ReplaceBlocks(page.ID, defaultAboutBlockSlice()); err != nil {
			return err
		}
	}
	return nil
}
