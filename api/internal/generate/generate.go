package generate

import (
	"encoding/json"
	"fmt"
	"html/template"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
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
	PathPrefix    string // "" for GHP publish; "/preview" for admin draft (nav/logo links)
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

func (g *Generator) buildFormatData(p cms.Page) template.JS {
	assets := make([]map[string]any, 0, len(p.Blocks))
	for _, b := range p.Blocks {
		if b.Type != cms.BlockGalleryImage {
			continue
		}
		var data map[string]any
		_ = json.Unmarshal(b.Data, &data)
		w, h := 1600, 2400
		if data != nil {
			mid, _ := data["media_id"].(string)
			url, _ := data["url"].(string)
			path := g.mediaFilePath(mid, url)
			if path != "" {
				if dw, dh, ok := probeImageSize(path); ok {
					w, h = dw, dh
				}
			}
		}
		assets = append(assets, map[string]any{
			"type":                       "image",
			"image_dimensions_1600x1200": []int{w, h},
		})
	}
	payload := map[string]any{
		"page": map[string]any{
			"type":   "gallery",
			"layout": "vertical",
			"title":  nil,
			"assets": assets,
		},
		"theme": map[string]any{
			"gallery_image_padding":       "Normal",
			"listing_thumbnail_size":      "Auto",
			"arrow_style":                 "Dark",
			"arrow_thickness":             "Medium",
			"gallery_change_image_speed":  "Normal",
			"gallery_full_height_mobile":  true,
			"menu_style":                  "Drop Down",
			"gallery_caption_typography": map[string]any{
				"background": "rgba(0,0,0,0.5)",
			},
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return template.JS(`{"page":{"type":"gallery","assets":[]},"theme":{"gallery_image_padding":"Normal"}}`)
	}
	return template.JS(raw)
}

func (g *Generator) mediaFilePath(id, url string) string {
	candidates := []string{}
	if id != "" {
		if m, err := g.Store.GetMedia(id); err == nil {
			candidates = append(candidates,
				filepath.Join(g.Cfg.UploadDir, m.Filename),
				filepath.Join(g.Cfg.ThemeSrc, "assets", "cdn", m.Filename),
			)
		}
	}
	if url != "" {
		fn := filepath.Base(strings.Split(url, "?")[0])
		candidates = append(candidates,
			filepath.Join(g.Cfg.UploadDir, fn),
			filepath.Join(g.Cfg.ThemeSrc, "assets", "cdn", fn),
			filepath.Join(g.Cfg.ThemeSrc, strings.TrimPrefix(url, "/")),
		)
	}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c
		}
	}
	return ""
}

func probeImageSize(path string) (int, int, bool) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, false
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil || cfg.Width < 1 || cfg.Height < 1 {
		return 0, 0, false
	}
	return cfg.Width, cfg.Height, true
}

// sitePath rewrites internal site links for draft preview (/preview/...) vs publish (/...).
func (g *Generator) sitePath(href string) string {
	prefix := strings.TrimRight(g.Cfg.PathPrefix, "/")
	h := strings.TrimSpace(href)
	if h == "" || h == "/" {
		if prefix == "" {
			return "/"
		}
		return prefix + "/"
	}
	if !strings.HasPrefix(h, "/") {
		h = "/" + h
	}
	if prefix == "" {
		// Prefer directory URLs for static hosting (GHP).
		if !strings.HasSuffix(h, "/") && h != "/" {
			return h + "/"
		}
		return h
	}
	out := prefix + h
	if !strings.HasSuffix(out, "/") {
		out += "/"
	}
	return out
}

func (g *Generator) prefixNav(items []cms.NavItem) []cms.NavItem {
	out := make([]cms.NavItem, len(items))
	for i, it := range items {
		cp := it
		if cp.Href != "" {
			cp.Href = g.sitePath(cp.Href)
		}
		if len(it.Children) > 0 {
			cp.Children = g.prefixNav(it.Children)
		}
		out[i] = cp
	}
	return out
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
	activeHref := g.sitePath("/" + active)

	name := p.Theme
	if name == "" {
		name = cms.ThemeTextContent
	}

	formatData := template.JS(`{"page":{"type":"gallery","layout":"vertical","title":null,"assets":[]},"theme":{"gallery_image_padding":"Normal","listing_thumbnail_size":"Auto"}}`)
	if name == cms.ThemePanoramaGallery {
		formatData = g.buildFormatData(p)
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
		"Nav":             g.prefixNav(nav),
		"HomeURL":         g.sitePath("/"),
		"ActiveSlug":      active,
		"ActiveHref":      activeHref,
		"MetaDescription": meta,
		"CanonicalURL":    canon,
		"RenderedBlocks":  blocks,
		"FormatData":      formatData,
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
<div class="_4ORMAT_content_page_row _4ormat_sort_item _4ORMAT_module_comparison_slider_01 format_comparison_slider" data-content-module-category="comparison-slider" style="--slider-default-position:50;--slider-color:#000;--slider-icon-color:#fff;--slider-line-thickness:2;--slider-size:48;--slider-icon-width:9px;--slider-icon-height:14px;--slider-icon-margin:6px;--slider-icon-shape:50%%;">
<div id="comparison_slider%s" data-dom-id="comparison_slider" data-editable-type="comparison-slider" data-using-default-images="false">
<div class="%s">
<div class="comparison_slider__slider_wrap">
<div class="comparison_slider__image_wrap">
<img alt="" class="comparison_slider__slider_image comparison_slider__slider_image--2" src="%s"/>
<img alt="" class="comparison_slider__slider_image comparison_slider__slider_image--1" src="%s"/>
</div>
<input class="comparison_slider__slider_range" max="1" min="0" name="slider" step="any" type="range" value="0.5" style="--slider-default-position:50"/>
<div class="comparison_slider__slider_button_container">
<div class="comparison_slider__slider_button comparison_slider__slider_button--chevrons">
<svg class="comparison_slider__slider_svg comparison_slider__slider_svg--left" viewBox="0 0 16 30"><path d="M15.16 30a.827.827 0 0 1-.584-.241L.256 15.615a.867.867 0 0 1 0-1.232L14.577.241a.828.828 0 0 1 1.188.02.87.87 0 0 1-.02 1.212L2.047 15l13.697 13.526a.87.87 0 0 1 .02 1.212.831.831 0 0 1-.604.262z"></path></svg>
<svg class="comparison_slider__slider_svg comparison_slider__slider_svg--right" viewBox="0 0 16 30"><path d="M.84 0c.21 0 .42.08.584.241l14.32 14.144a.867.867 0 0 1 0 1.232L1.423 29.759a.828.828 0 0 1-1.188-.02.87.87 0 0 1 .02-1.212L13.953 15 .256 1.474A.87.87 0 0 1 .236.262.831.831 0 0 1 .84 0z"></path></svg>
</div>
</div>
</div>
<div class="comparison_slider__copy_wrap"></div>
</div>
</div>
</div>`, uid, cls, template.HTMLEscapeString(after), template.HTMLEscapeString(before))

		case cms.BlockGalleryImage:
			mid, _ := data["media_id"].(string)
			url, _ := data["url"].(string)
			alt, _ := data["alt"].(string)
			src, _ := resolve(mid, url)
			html = fmt.Sprintf(`
<div class="asset image">
<div class="wrap">
<div class="img">
<span class="midd hide-for-large"></span>
<img alt="%s" class="lazyload" data-pin-nopin="true" loading="eager" src="%s" data-src="%s"/>
</div>
</div>
</div>`, template.HTMLEscapeString(alt), template.HTMLEscapeString(src), template.HTMLEscapeString(src))

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
