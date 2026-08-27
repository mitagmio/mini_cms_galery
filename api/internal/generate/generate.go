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
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"sheyanova.art/api/internal/cms"
)

type Config struct {
	OutDir           string
	UploadDir        string
	ThemeSrc         string // FRONT_THEME_SRC or path to front/
	PreviewBase      string // URL prefix for preview, e.g. /preview
	PathPrefix       string // "" for GHP publish; "/preview" for admin draft (nav/logo links)
	CanonicalBase    string
	PublicAPIURL     string
	TurnstileSiteKey string
	PublishedOnly    bool // true for live publish: skip drafts and unpublished nav links
}

type Generator struct {
	Store       *cms.Store
	Cfg         Config
	tmpl        *template.Template
	pagesByID   map[string]cms.Page
	pagesBySlug map[string]cms.Page
	srcByTheme  map[string]string
	formsByKey  map[string]cms.Template
	// imgDims caches DecodeConfig results for one generate run (renderBlocks + buildFormatData).
	imgDims map[string]imgDim
}

type imgDim struct {
	w, h int
	ok   bool
}

type renderedBlock struct {
	HTML     template.HTML
	ImageSrc string
	Alt      string
	Type     string
	Width    int
	Height   int
}

const galleryLoadMarkHTML = `<span class="gallery-load-mark" aria-hidden="true"><span class="gallery-load-line"></span></span>`

func New(store *cms.Store, cfg Config) (*Generator, error) {
	if cfg.OutDir == "" {
		cfg.OutDir = "/data/preview"
	}
	if cfg.PreviewBase == "" {
		cfg.PreviewBase = "/preview"
	}
	tmpl, err := template.New("root").Funcs(templateFuncMap()).ParseFS(templateFS, "templates/*.gohtml")
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}
	return &Generator{Store: store, Cfg: cfg, tmpl: tmpl}, nil
}

// GenerateSite writes the full static site into OutDir.
func (g *Generator) GenerateSite() error {
	g.imgDims = nil
	if err := os.MkdirAll(g.Cfg.OutDir, 0o755); err != nil {
		return err
	}
	if err := g.copyThemeAssets(); err != nil {
		return err
	}
	if err := g.writeArticleCSS(); err != nil {
		return err
	}
	if err := g.writePagesMeta(); err != nil {
		return err
	}
	pages, err := g.Store.ListPages()
	if err != nil {
		return err
	}
	g.setPageIndex(pages)
	g.loadTemplateOverrides()
	emitted := make([]cms.Page, 0, len(pages))
	for _, p := range pages {
		if g.Cfg.PublishedOnly && !p.IsPublished() {
			continue
		}
		full, err := g.Store.GetPage(p.ID)
		if err != nil {
			return err
		}
		if err := g.writePage(full); err != nil {
			return fmt.Errorf("page %s: %w", full.Slug, err)
		}
		emitted = append(emitted, full)
	}
	return g.pruneObsoletePageDirs(emitted)
}

// pruneObsoletePageDirs removes /{old-slug}/ dirs left after renames so preview
// does not keep stale pages (e.g. empty BA range UI under a previous slug).
func (g *Generator) pruneObsoletePageDirs(pages []cms.Page) error {
	keep := map[string]bool{
		"assets": true,
		"static": true,
	}
	for _, p := range pages {
		if s := strings.Trim(p.Slug, "/"); s != "" {
			keep[s] = true
		}
	}
	entries, err := os.ReadDir(g.Cfg.OutDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if keep[name] || strings.HasPrefix(name, ".") {
			continue
		}
		index := filepath.Join(g.Cfg.OutDir, name, "index.html")
		if _, err := os.Stat(index); err != nil {
			continue
		}
		_ = os.RemoveAll(filepath.Join(g.Cfg.OutDir, name))
	}
	return nil
}

// GeneratePage writes a single page and returns its public preview URL path.
func (g *Generator) GeneratePage(pageID string) (string, error) {
	g.imgDims = nil
	if err := os.MkdirAll(g.Cfg.OutDir, 0o755); err != nil {
		return "", err
	}
	_ = g.copyThemeAssets()
	_ = g.writeArticleCSS()
	p, err := g.Store.GetPage(pageID)
	if err != nil {
		return "", err
	}
	if pages, err := g.Store.ListPages(); err == nil {
		g.setPageIndex(pages)
	}
	g.loadTemplateOverrides()
	if err := g.writePage(p); err != nil {
		return "", err
	}
	return g.pageURL(p), nil
}

func (g *Generator) setPageIndex(pages []cms.Page) {
	g.pagesByID = make(map[string]cms.Page, len(pages))
	g.pagesBySlug = make(map[string]cms.Page, len(pages))
	for _, p := range pages {
		g.pagesByID[p.ID] = p
		if slug := strings.Trim(p.Slug, "/"); slug != "" {
			g.pagesBySlug[slug] = p
		}
		if p.IsHomepage {
			g.pagesBySlug[""] = p
		}
	}
}

func (g *Generator) loadTemplateOverrides() {
	g.srcByTheme = map[string]string{}
	g.formsByKey = map[string]cms.Template{}
	if g.Store == nil {
		return
	}
	list, err := g.Store.ListTemplates()
	if err != nil {
		log.Printf("generate: list templates: %v", err)
		return
	}
	for _, t := range list {
		if t.Kind == cms.TemplateKindForm && t.FormKey != "" {
			g.formsByKey[t.FormKey] = t
			continue
		}
		src := strings.TrimSpace(t.Source)
		if src == "" || t.Theme == "" {
			continue
		}
		if t.ID == t.Theme || t.IsSystem {
			g.srcByTheme[t.Theme] = src
		}
	}
}

func (g *Generator) executePageTemplate(name string, view any, buf *strings.Builder) error {
	src := ""
	if g.srcByTheme != nil {
		src = strings.TrimSpace(g.srcByTheme[name])
	}
	if src != "" {
		clone, err := g.tmpl.Clone()
		if err != nil {
			log.Printf("generate: clone templates for %s: %v", name, err)
		} else if _, err := clone.Parse(src); err != nil {
			log.Printf("generate: db source %s parse failed, falling back to file: %v", name, err)
		} else if err := clone.ExecuteTemplate(buf, name, view); err != nil {
			log.Printf("generate: db source %s execute failed, falling back to file: %v", name, err)
			buf.Reset()
		} else {
			return nil
		}
		buf.Reset()
	}
	return g.tmpl.ExecuteTemplate(buf, name, view)
}

func (g *Generator) contactAction() string {
	api := strings.TrimRight(strings.TrimSpace(g.Cfg.PublicAPIURL), "/")
	if api == "" {
		api = "https://api.sheyanova.art"
	}
	return api + "/api/contact"
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
		if !galleryDataHasMedia(data) {
			continue
		}
		w, h := 1600, 2400
		if data != nil {
			mid, _ := data["media_id"].(string)
			url, _ := data["url"].(string)
			path := g.mediaFilePath(mid, url)
			if path != "" {
				if dw, dh, ok := g.cachedImageSize(path); ok {
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
			"gallery_image_padding":      "Normal",
			"listing_thumbnail_size":     "Auto",
			"arrow_style":                "Dark",
			"arrow_thickness":            "Medium",
			"gallery_change_image_speed": "Normal",
			"gallery_full_height_mobile": true,
			"menu_style":                 "Drop Down",
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

func (g *Generator) cachedImageSize(path string) (int, int, bool) {
	if path == "" {
		return 0, 0, false
	}
	if g.imgDims != nil {
		if d, ok := g.imgDims[path]; ok {
			return d.w, d.h, d.ok
		}
	} else {
		g.imgDims = make(map[string]imgDim)
	}
	w, h, ok := probeImageSize(path)
	g.imgDims[path] = imgDim{w: w, h: h, ok: ok}
	return w, h, ok
}

func (g *Generator) probeMediaSize(id, url string) (int, int) {
	path := g.mediaFilePath(id, url)
	if path == "" {
		return 0, 0
	}
	w, h, ok := g.cachedImageSize(path)
	if !ok {
		return 0, 0
	}
	return w, h
}

func imgDimensionAttrs(w, h int) string {
	if w < 1 || h < 1 {
		return ""
	}
	return fmt.Sprintf(` width="%d" height="%d"`, w, h)
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
	out := make([]cms.NavItem, 0, len(items))
	for _, it := range items {
		if !it.Visible {
			continue
		}
		cp := it
		if len(it.Children) > 0 {
			cp.Children = g.prefixNav(it.Children)
		} else {
			cp.Children = []cms.NavItem{}
		}
		if cp.Kind == cms.NavKindCategory && len(cp.Children) == 0 {
			continue
		}
		if !g.navLinkAllowed(cp) {
			continue
		}
		if cp.Href != "" && !isExternalHref(cp.Href) {
			cp.Href = g.sitePath(cp.Href)
		}
		out = append(out, cp)
	}
	return out
}

func isExternalHref(h string) bool {
	h = strings.TrimSpace(h)
	return strings.HasPrefix(h, "http://") || strings.HasPrefix(h, "https://") ||
		strings.HasPrefix(h, "mailto:") || strings.HasPrefix(h, "//") || strings.HasPrefix(h, "#")
}

func (g *Generator) navLinkAllowed(it cms.NavItem) bool {
	if !g.Cfg.PublishedOnly {
		return true
	}
	if it.Kind == cms.NavKindCategory {
		return true
	}
	if isExternalHref(it.Href) {
		return true
	}
	if it.PageID != "" {
		p, ok := g.pagesByID[it.PageID]
		if !ok {
			return false
		}
		return p.IsPublished()
	}
	slug := strings.Trim(it.Href, "/")
	if slug == "" {
		if p, ok := g.pagesBySlug[""]; ok {
			return p.IsPublished()
		}
		return true
	}
	if p, ok := g.pagesBySlug[slug]; ok {
		return p.IsPublished()
	}
	return true
}

func pageCanonicalURL(p cms.Page, canonBase string) string {
	path := strings.TrimSpace(p.CanonicalPath)
	if path == "" && p.SEO != nil {
		path = strings.TrimSpace(p.SEO.CanonicalPath)
	}
	if path != "" {
		if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
			return path
		}
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		return canonBase + path
	}
	if p.IsHomepage {
		return canonBase + "/"
	}
	return canonBase + "/" + strings.Trim(p.Slug, "/")
}

func (g *Generator) writePage(p cms.Page) error {
	if g.formsByKey == nil && g.srcByTheme == nil {
		g.loadTemplateOverrides()
	}
	settings, err := g.Store.GetSettings()
	if err != nil {
		return err
	}
	nav, err := g.Store.GetNavTree()
	if err != nil {
		return err
	}
	name := p.Theme
	if name == "" {
		name = cms.ThemeTextContent
	}

	shuffleSeed := int64(0)
	renderPage := p
	if name == cms.ThemeLookbookGallery {
		seed, err := g.ensureLookbookShuffleSeed(&p)
		if err != nil {
			return err
		}
		shuffleSeed = seed
		renderPage.Blocks = permuteGalleryBlocks(p.Blocks, seed)
	}

	blocks, mediaUsed, err := g.renderBlocks(renderPage)
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
	canon := pageCanonicalURL(p, canonBase)
	htmlTitle := strings.TrimSpace(p.MetaTitle)
	if htmlTitle == "" && p.SEO != nil {
		htmlTitle = strings.TrimSpace(p.SEO.MetaTitle)
	}
	if htmlTitle == "" {
		htmlTitle = p.Title
	}

	active := p.Slug
	if p.IsHomepage {
		active = "before-after"
	}
	activeHref := g.sitePath("/" + active)

	formatData := template.JS(`{"page":{"type":"gallery","layout":"vertical","title":null,"assets":[]},"theme":{"gallery_image_padding":"Normal","listing_thumbnail_size":"Auto"}}`)
	if name == cms.ThemePanoramaGallery || name == cms.ThemeLookbookGallery {
		formatData = g.buildFormatData(renderPage)
	}

	portrait, bio, aboutRest := splitAboutHero(blocks)
	hasPhoto := strings.TrimSpace(string(portrait)) != ""
	hasBio := strings.TrimSpace(string(bio)) != ""

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
		"Nav":              g.prefixNav(nav),
		"HomeURL":          g.sitePath("/"),
		"ActiveSlug":       active,
		"ActiveHref":       activeHref,
		"MetaDescription":  meta,
		"CanonicalURL":     canon,
		"HTMLTitle":        htmlTitle,
		"RenderedBlocks":   blocks,
		"FormatData":       formatData,
		"ShuffleSeed":      shuffleSeed,
		"RateModals":       g.rateModals(p),
		"BannerGridStyle":  template.CSS(cms.BannerGridStyle(p.Settings)),
		"TurnstileSiteKey": g.Cfg.TurnstileSiteKey,
		"AboutPortrait":    portrait,
		"AboutBio":         bio,
		"AboutRest":        aboutRest,
		"AboutHasPhoto":    hasPhoto,
		"AboutHasBio":      hasBio,
		"AboutHeroClass":   aboutHeroClass(hasPhoto, hasBio),
	}

	var buf strings.Builder
	if err := g.executePageTemplate(name, view, &buf); err != nil {
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

	firstComparison := true
	firstArticleImage := true
	articleTheme := cms.IsArticleTheme(p.Theme)
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
			// Empty BA pair: skip — native range input otherwise shows as a blue strip
			// (gallery pages don't load comparison_slider CSS).
			if strings.TrimSpace(before) == "" && strings.TrimSpace(after) == "" {
				continue
			}
			overlay, _ := data["overlay"].(bool)
			cls := "comparison_slider ba-pair ba-await"
			if overlay {
				cls += " comparison_slider--overlay"
			}
			afterSrc := pathURL(after)
			beforeSrc := pathURL(before)
			dim := imgDimensionAttrs(g.probeMediaSize(beforeID, beforeURL))
			if dim == "" {
				dim = imgDimensionAttrs(g.probeMediaSize(afterID, afterURL))
			}
			var afterTag, beforeTag string
			if firstComparison {
				cls += " ba-first"
				afterTag = fmt.Sprintf(`<img alt="" class="comparison_slider__slider_image comparison_slider__slider_image--2" decoding="async" loading="eager" fetchpriority="high" src="%s" data-src="%s"%s/>`, afterSrc, afterSrc, dim)
				beforeTag = fmt.Sprintf(`<img alt="" class="comparison_slider__slider_image comparison_slider__slider_image--1" decoding="async" loading="eager" fetchpriority="high" src="%s" data-src="%s"%s/>`, beforeSrc, beforeSrc, dim)
				firstComparison = false
			} else {
				afterTag = fmt.Sprintf(`<img alt="" class="comparison_slider__slider_image comparison_slider__slider_image--2" decoding="async" loading="lazy" fetchpriority="low" src="%s" data-src="%s"%s/>`, afterSrc, afterSrc, dim)
				beforeTag = fmt.Sprintf(`<img alt="" class="comparison_slider__slider_image comparison_slider__slider_image--1" decoding="async" loading="lazy" fetchpriority="low" src="%s" data-src="%s"%s/>`, beforeSrc, beforeSrc, dim)
			}
			uid := fmt.Sprintf("%s-%d", b.ID, i)
			html = fmt.Sprintf(`
<div class="_4ORMAT_content_page_row _4ormat_sort_item _4ORMAT_module_comparison_slider_01 format_comparison_slider" data-content-module-category="comparison-slider" style="--slider-default-position:50;--slider-color:#000;--slider-icon-color:#fff;--slider-line-thickness:2;--slider-size:48;--slider-icon-width:9px;--slider-icon-height:14px;--slider-icon-margin:6px;--slider-icon-shape:50%%;">
<div id="comparison_slider%s" data-dom-id="comparison_slider" data-editable-type="comparison-slider" data-using-default-images="false">
<div class="%s">
<div class="comparison_slider__slider_wrap">
<div class="comparison_slider__image_wrap">
%s
%s
%s
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
</div>`, uid, cls, galleryLoadMarkHTML, afterTag, beforeTag)

		case cms.BlockGalleryImage:
			mid, _ := data["media_id"].(string)
			url, _ := data["url"].(string)
			alt, _ := data["alt"].(string)
			caption, _ := data["caption"].(string)
			src, _ := resolve(mid, url)
			if strings.TrimSpace(src) == "" {
				continue
			}
			imgSrc := pathURL(src)
			w, h := g.probeMediaSize(mid, url)
			dim := imgDimensionAttrs(w, h)
			if articleTheme {
				loading := "lazy"
				extra := ""
				if firstArticleImage {
					loading = "eager"
					firstArticleImage = false
					if p.Theme == cms.ThemeAboutContent {
						extra = "about-portrait"
					}
				}
				html = articleFigureHTML(imgSrc, alt, caption, dim, loading, extra)
				if p.Theme == cms.ThemeTextContent {
					html = fmt.Sprintf(`
<div class="_4ORMAT_content_page_row _4ormat_sort_item">
<div class="eightcol">
%s
</div>
</div>`, html)
				}
				out = append(out, renderedBlock{HTML: template.HTML(html), ImageSrc: imgSrc, Alt: alt, Type: cms.BlockGalleryImage, Width: w, Height: h})
				continue
			}
			html = fmt.Sprintf(`
<div class="asset image asset-await">
<div class="wrap">
<div class="img">
%s
<span class="midd hide-for-large"></span>
<img alt="%s" class="gallery-photo" data-pin-nopin="true" decoding="async"%s data-src="%s"/>
</div>
</div>
</div>`, galleryLoadMarkHTML, template.HTMLEscapeString(alt), dim, imgSrc)
			out = append(out, renderedBlock{HTML: template.HTML(html), ImageSrc: imgSrc, Alt: alt, Type: cms.BlockGalleryImage, Width: w, Height: h})
			continue

		case cms.BlockRichText:
			raw, _ := data["html"].(string)
			if p.Theme == cms.ThemeAboutContent {
				html = fmt.Sprintf(`<div class="rich-text about-copy">%s</div>`, raw)
			} else {
				html = fmt.Sprintf(`
<div class="_4ORMAT_content_page_row _4ormat_sort_item">
<div class="eightcol">
<div class="rich-text">%s</div>
</div>
</div>`, raw)
			}

		case cms.BlockContactForm:
			nameL, _ := data["name_label"].(string)
			emailL, _ := data["email_label"].(string)
			msgL, _ := data["message_label"].(string)
			subL, _ := data["submit_label"].(string)
			success, _ := data["success_message"].(string)
			formID := "contact_form"
			if b.ID != "" {
				formID = "contact_form_" + b.ID
			}
			html = RenderContactForm(ContactFormInput{
				APIURL:           g.Cfg.PublicAPIURL,
				FormID:           formID,
				NameLabel:        nameL,
				EmailLabel:       emailL,
				MessageLabel:     msgL,
				SubmitLabel:      subL,
				SuccessMessage:   success,
				TurnstileSiteKey: g.Cfg.TurnstileSiteKey,
			})

		case cms.BlockRateBanner:
			mid, _ := data["media_id"].(string)
			url, _ := data["url"].(string)
			src, _ := resolve(mid, url)
			html = renderRateBanner(data, src)

		default:
			html = fmt.Sprintf(`<!-- unknown block type %s -->`, template.HTMLEscapeString(b.Type))
		}
		out = append(out, renderedBlock{HTML: template.HTML(html), Type: b.Type})
	}
	return out, used, nil
}

// pathURL percent-encodes each path segment so Cyrillic filenames work on GitHub Pages.
func pathURL(p string) string {
	if p == "" || strings.HasPrefix(p, "http://") || strings.HasPrefix(p, "https://") || strings.HasPrefix(p, "data:") {
		return template.HTMLEscapeString(p)
	}
	parts := strings.Split(p, "/")
	for i, seg := range parts {
		if seg == "" {
			continue
		}
		parts[i] = url.PathEscape(seg)
	}
	return strings.Join(parts, "/")
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
	srcAbs, err := filepath.Abs(src)
	if err != nil {
		srcAbs = src
	}
	outAbs, err := filepath.Abs(g.Cfg.OutDir)
	if err != nil {
		outAbs = g.Cfg.OutDir
	}
	// Publish uses ThemeSrc == OutDir (/front → /front). Copying a file onto itself
	// truncates it to 0 bytes — skip the whole tree in that case.
	if srcAbs == outAbs {
		return nil
	}
	// Theme kit only (~2MB): assets/theme + other non-cdn asset dirs, static, fonts.
	// Never bulk-copy assets/cdn (~190MB); page images come from copyMedia.
	dirs := themeKitDirs(src)
	for _, d := range dirs {
		from := filepath.Join(src, d)
		to := filepath.Join(g.Cfg.OutDir, d)
		if _, err := os.Stat(from); err != nil {
			continue
		}
		if err := copyDirSkipUnchanged(from, to); err != nil {
			return fmt.Errorf("copy %s: %w", d, err)
		}
	}
	return nil
}

// themeKitDirs lists relative dirs to copy for draft generate (excludes assets/cdn).
func themeKitDirs(themeSrc string) []string {
	dirs := []string{filepath.Join("assets", "theme"), "static", "fonts"}
	assetsRoot := filepath.Join(themeSrc, "assets")
	entries, err := os.ReadDir(assetsRoot)
	if err != nil {
		return dirs
	}
	seen := map[string]bool{"theme": true}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if name == "cdn" || seen[name] {
			continue
		}
		seen[name] = true
		dirs = append(dirs, filepath.Join("assets", name))
	}
	return dirs
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

func copyDirSkipUnchanged(src, dst string) error {
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
		return copyFileSkipUnchanged(path, target)
	})
}

func copyFile(src, dst string) error {
	absSrc, err1 := filepath.Abs(src)
	absDst, err2 := filepath.Abs(dst)
	if err1 == nil && err2 == nil && absSrc == absDst {
		return nil
	}
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

// copyFileSkipUnchanged copies src→dst unless dst already matches size+mtime.
// Preserves source mtime on the destination so subsequent GeneratePage stays cheap.
func copyFileSkipUnchanged(src, dst string) error {
	absSrc, err1 := filepath.Abs(src)
	absDst, err2 := filepath.Abs(dst)
	if err1 == nil && err2 == nil && absSrc == absDst {
		return nil
	}
	si, err := os.Stat(src)
	if err != nil {
		return err
	}
	if di, err := os.Stat(dst); err == nil && !di.IsDir() {
		if di.Size() == si.Size() && di.ModTime().Equal(si.ModTime()) {
			return nil
		}
	}
	if err := copyFile(src, dst); err != nil {
		return err
	}
	return os.Chtimes(dst, si.ModTime(), si.ModTime())
}
