package generate

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"sheyanova.art/api/internal/cms"
)

type Config struct {
	OutDir        string
	UploadDir     string
	ThemeSrc      string // FRONT_THEME_SRC or path to front/
	PreviewBase   string // URL prefix for preview, e.g. /preview
	CanonicalBase string
}

type Generator struct {
	Store *cms.Store
	Cfg   Config
	tmpl  *template.Template
}

type renderedBlock struct {
	HTML template.HTML
}

func New(store *cms.Store, cfg Config) (*Generator, error) {
	if cfg.OutDir == "" {
		cfg.OutDir = "/data/preview"
	}
	if cfg.PreviewBase == "" {
		cfg.PreviewBase = "/preview"
	}
	funcs := template.FuncMap{
		"trimSlash": func(s string) string {
			return strings.Trim(strings.TrimPrefix(s, "/"), "/")
		},
	}
	tmpl, err := template.New("root").Funcs(funcs).ParseFS(templateFS, "templates/*.gohtml")
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}
	return &Generator{Store: store, Cfg: cfg, tmpl: tmpl}, nil
}

// GenerateSite writes the full static site into OutDir.
func (g *Generator) GenerateSite() error {
	if err := os.MkdirAll(g.Cfg.OutDir, 0o755); err != nil {
		return err
	}
	if err := g.copyThemeAssets(); err != nil {
		return err
	}
	if err := g.writePagesMeta(); err != nil {
		return err
	}
	pages, err := g.Store.ListPages()
	if err != nil {
		return err
	}
	for _, p := range pages {
		full, err := g.Store.GetPage(p.ID)
		if err != nil {
			return err
		}
		if err := g.writePage(full); err != nil {
			return fmt.Errorf("page %s: %w", full.Slug, err)
		}
	}
	return nil
}

// GeneratePage writes a single page and returns its public preview URL path.
func (g *Generator) GeneratePage(pageID string) (string, error) {
	if err := os.MkdirAll(g.Cfg.OutDir, 0o755); err != nil {
		return "", err
	}
	_ = g.copyThemeAssets()
	p, err := g.Store.GetPage(pageID)
	if err != nil {
		return "", err
	}
	if err := g.writePage(p); err != nil {
		return "", err
	}
	return g.pageURL(p), nil
}

func (g *Generator) pageURL(p cms.Page) string {
	base := strings.TrimRight(g.Cfg.PreviewBase, "/")
	if p.IsHomepage {
		return base + "/"
	}
	return base + "/" + p.Slug + "/"
}

func (g *Generator) writePage(p cms.Page) error {
	settings, err := g.Store.GetSettings()
	if err != nil {
		return err
	}
	nav, err := g.Store.GetNavTree()
	if err != nil {
		return err
	}
	blocks, mediaUsed, err := g.renderBlocks(p)
	if err != nil {
		return err
	}
	for _, m := range mediaUsed {
		if err := g.copyMedia(m); err != nil {
			return err
		}
	}

	meta := p.MetaDescription
	if meta == "" {
		meta = settings.Description
	}
	canonBase := settings.CanonicalBase
	if g.Cfg.CanonicalBase != "" {
		canonBase = g.Cfg.CanonicalBase
	}
	canonBase = strings.TrimRight(canonBase, "/")
	canon := canonBase + "/"
	if !p.IsHomepage {
		canon = canonBase + "/" + p.Slug
	}

	active := p.Slug
	if p.IsHomepage {
		active = "before-after"
	}

	view := map[string]any{
		"Page": p,
		"Settings": map[string]any{
			"SiteName":     settings.SiteName,
			"LogoHTML":     template.HTML(settings.LogoHTML),
			"Description":  settings.Description,
			"InstagramURL": settings.InstagramURL,
			"BehanceURL":   settings.BehanceURL,
			"LinkedInURL":  settings.LinkedInURL,
			"Copyright":    settings.Copyright,
		},
		"Nav":             nav,
		"ActiveSlug":      active,
		"MetaDescription": meta,
		"CanonicalURL":    canon,
		"RenderedBlocks":  blocks,
	}

	name := p.Theme
	if name == "" {
		name = cms.ThemeTextContent
	}
	var buf strings.Builder
	if err := g.tmpl.ExecuteTemplate(&buf, name, view); err != nil {
		return err
	}

	html := []byte(buf.String())
	// Always write /{slug}/index.html so nav links like /before-after work.
	if p.Slug != "" {
		dir := filepath.Join(g.Cfg.OutDir, p.Slug)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, "index.html"), html, 0o644); err != nil {
			return err
		}
	}
	// Homepage also at site root.
	if p.IsHomepage {
		return os.WriteFile(filepath.Join(g.Cfg.OutDir, "index.html"), html, 0o644)
	}
	return nil
}

func (g *Generator) renderBlocks(p cms.Page) ([]renderedBlock, []cms.Media, error) {
	out := make([]renderedBlock, 0, len(p.Blocks))
	used := make([]cms.Media, 0)
	seen := map[string]bool{}

	resolve := func(id, url string) (string, error) {
		if id != "" {
			m, err := g.Store.GetMedia(id)
			if err == nil {
				if !seen[m.ID] {
					seen[m.ID] = true
					used = append(used, m)
				}
				return "/assets/cdn/" + m.Filename, nil
			}
		}
		if url == "" {
			return "", nil
		}
		if strings.HasPrefix(url, "/media/") {
			filename := strings.TrimPrefix(url, "/media/")
			m := cms.Media{ID: filename, Filename: filename, URL: url}
			if !seen[m.Filename] {
				seen[m.Filename] = true
				used = append(used, m)
			}
			return "/assets/cdn/" + filename, nil
		}
		if strings.HasPrefix(url, "/assets/cdn/") {
			filename := strings.TrimPrefix(url, "/assets/cdn/")
			m := cms.Media{ID: filename, Filename: filename, URL: url}
			if !seen[m.Filename] {
				seen[m.Filename] = true
				used = append(used, m)
			}
			return url, nil
		}
		return url, nil
	}

	for i, b := range p.Blocks {
		var data map[string]any
		_ = json.Unmarshal(b.Data, &data)
		if data == nil {
			data = map[string]any{}
		}
		var html string
		switch b.Type {
		case cms.BlockComparisonSlider:
			beforeID, _ := data["before_media_id"].(string)
			afterID, _ := data["after_media_id"].(string)
			beforeURL, _ := data["before_url"].(string)
			afterURL, _ := data["after_url"].(string)
			before, _ := resolve(beforeID, beforeURL)
			after, _ := resolve(afterID, afterURL)
			overlay, _ := data["overlay"].(bool)
			cls := "comparison_slider"
			if overlay {
				cls += " comparison_slider--overlay"
			}
			uid := fmt.Sprintf("%s-%d", b.ID, i)
			html = fmt.Sprintf(`
<div class="_4ORMAT_content_page_row _4ormat_sort_item format_comparison_slider" data-content-module-category="comparison-slider">
<div id="comparison_slider%s" data-editable-type="comparison-slider">
<div class="%s">
<div class="comparison_slider__slider_wrap">
<div class="comparison_slider__image_wrap">
<img alt="" class="comparison_slider__slider_image comparison_slider__slider_image--2" src="%s"/>
<img alt="" class="comparison_slider__slider_image comparison_slider__slider_image--1" src="%s"/>
</div>
<input class="comparison_slider__slider_range" max="1" min="0" name="slider" step="any" type="range" value="0.5"/>
<div class="comparison_slider__slider_button_container">
<div class="comparison_slider__slider_button comparison_slider__slider_button--chevrons"></div>
</div>
</div>
</div>
</div>
</div>`, uid, cls, template.HTMLEscapeString(after), template.HTMLEscapeString(before))

		case cms.BlockGalleryImage:
			mid, _ := data["media_id"].(string)
			url, _ := data["url"].(string)
			alt, _ := data["alt"].(string)
			src, _ := resolve(mid, url)
			html = fmt.Sprintf(`
<div class="asset image" data-asset-type="image">
<img src="%s" alt="%s" loading="lazy"/>
</div>`, template.HTMLEscapeString(src), template.HTMLEscapeString(alt))

		case cms.BlockRichText:
			raw, _ := data["html"].(string)
			html = fmt.Sprintf(`
<div class="_4ORMAT_content_page_row _4ormat_sort_item">
<div class="eightcol">
<div class="rich-text">%s</div>
</div>
</div>`, raw)

		case cms.BlockContactForm:
			nameL, _ := data["name_label"].(string)
			emailL, _ := data["email_label"].(string)
			msgL, _ := data["message_label"].(string)
			subL, _ := data["submit_label"].(string)
			if nameL == "" {
				nameL = "Name"
			}
			if emailL == "" {
				emailL = "Email"
			}
			if msgL == "" {
				msgL = "Message"
			}
			if subL == "" {
				subL = "Send Message"
			}
			html = fmt.Sprintf(`
<div class="_4ORMAT_content_page_row _4ormat_sort_item">
<form class="contact_form" method="post" action="#" onsubmit="return false;">
<label>%s<input type="text" name="name" required/></label>
<label>%s<input type="email" name="email" required/></label>
<label>%s<textarea name="message" required></textarea></label>
<button type="submit">%s</button>
</form>
</div>`, template.HTMLEscapeString(nameL), template.HTMLEscapeString(emailL),
				template.HTMLEscapeString(msgL), template.HTMLEscapeString(subL))

		default:
			html = fmt.Sprintf(`<!-- unknown block type %s -->`, template.HTMLEscapeString(b.Type))
		}
		out = append(out, renderedBlock{HTML: template.HTML(html)})
	}
	return out, used, nil
}

func (g *Generator) copyMedia(m cms.Media) error {
	dstDir := filepath.Join(g.Cfg.OutDir, "assets", "cdn")
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return err
	}
	dst := filepath.Join(dstDir, m.Filename)
	candidates := []string{
		filepath.Join(g.Cfg.UploadDir, m.Filename),
	}
	if g.Cfg.ThemeSrc != "" {
		candidates = append(candidates,
			filepath.Join(g.Cfg.ThemeSrc, "assets", "cdn", m.Filename),
			filepath.Join(g.Cfg.ThemeSrc, strings.TrimPrefix(m.URL, "/")),
		)
	}
	for _, src := range candidates {
		if src == "" {
			continue
		}
		if err := copyFile(src, dst); err == nil {
			return nil
		}
	}
	// missing media is non-fatal for draft generate
	return nil
}

func (g *Generator) writePagesMeta() error {
	// Disable Jekyll so assets/_ folders are served on GitHub Pages.
	if err := os.WriteFile(filepath.Join(g.Cfg.OutDir, ".nojekyll"), []byte{}, 0o644); err != nil {
		return err
	}
	host := strings.TrimSpace(g.Cfg.CanonicalBase)
	host = strings.TrimPrefix(host, "https://")
	host = strings.TrimPrefix(host, "http://")
	host = strings.Trim(host, "/")
	if host == "" {
		host = "sheyanova.art"
	}
	return os.WriteFile(filepath.Join(g.Cfg.OutDir, "CNAME"), []byte(host+"\n"), 0o644)
}

func (g *Generator) copyThemeAssets() error {
	src := g.Cfg.ThemeSrc
	if src == "" {
		return nil
	}
	dirs := []string{"assets", "static", "fonts"}
	for _, d := range dirs {
		from := filepath.Join(src, d)
		to := filepath.Join(g.Cfg.OutDir, d)
		if _, err := os.Stat(from); err != nil {
			continue
		}
		if err := copyDir(from, to); err != nil {
			return fmt.Errorf("copy %s: %w", d, err)
		}
	}
	return nil
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
