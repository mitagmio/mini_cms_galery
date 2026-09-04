package generate

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"

	"sheyanova.art/api/internal/cms"
)

const defaultRobotsMeta = "noai, noimageai"

// Static favicon assets already present in the Format theme kit / front root.
var defaultFaviconLinks = []struct {
	Rel, Href, Type, Sizes string
}{
	{Rel: "icon", Href: "/favicon.ico", Sizes: "any"},
	{Rel: "icon", Href: "/static/favicon-32-7b151b5cb1ea57453cf4f6e4dca6e59f40b326568045ed7ee8e2da4ad0096e63.png", Type: "image/png", Sizes: "32x32"},
	{Rel: "icon", Href: "/static/favicon-96-fd00130996e46dd74db83867237ee7b02e48d37456ed20a348ee43b5c6b6b576.png", Type: "image/png", Sizes: "96x96"},
	{Rel: "apple-touch-icon", Href: "/static/favicon-192-36a03c4d1a4754694fea4f56150aa9d363ab3cbcc0dae286f87e3f47979c55f9.png", Sizes: "192x192"},
}

type pageSEOView struct {
	Keywords        string
	Robots          string
	OGTitle         string
	OGDescription   string
	OGType          string
	OGURL           string
	OGImage         string
	OGSiteName      string
	TwitterCard     string
	FaviconHTML     template.HTML
	PageHeading     string
	ShowPageHeading bool
}

func (g *Generator) buildPageSEO(p cms.Page, settings cms.SiteSettings, canonBase, canon, htmlTitle, metaDesc string) pageSEOView {
	siteName := strings.TrimSpace(settings.SiteName)
	if siteName == "" {
		siteName = "Daria Sheyanova"
	}

	ogTitle := strings.TrimSpace(p.OGTitle)
	if ogTitle == "" && p.SEO != nil {
		ogTitle = strings.TrimSpace(p.SEO.OGTitle)
	}
	if ogTitle == "" {
		ogTitle = htmlTitle
	}

	ogDesc := strings.TrimSpace(p.OGDescription)
	if ogDesc == "" && p.SEO != nil {
		ogDesc = strings.TrimSpace(p.SEO.OGDescription)
	}
	if ogDesc == "" {
		ogDesc = metaDesc
	}

	ogType := strings.TrimSpace(p.OGType)
	if ogType == "" && p.SEO != nil {
		ogType = strings.TrimSpace(p.SEO.OGType)
	}
	if ogType == "" {
		ogType = "website"
	}

	keywords := strings.TrimSpace(p.MetaKeywords)
	if keywords == "" && p.SEO != nil {
		keywords = strings.TrimSpace(p.SEO.MetaKeywords)
	}
	if keywords == "" {
		keywords = strings.TrimSpace(settings.DefaultKeywords)
	}

	robots := strings.TrimSpace(settings.Robots)
	if robots == "" {
		robots = defaultRobotsMeta
	}

	ogImageID := strings.TrimSpace(p.OGImage)
	if ogImageID == "" && p.SEO != nil {
		ogImageID = strings.TrimSpace(p.SEO.OGImageMediaID)
	}
	if ogImageID == "" {
		ogImageID = strings.TrimSpace(settings.OGImageMediaID)
	}
	if ogImageID == "" {
		ogImageID = strings.TrimSpace(settings.OGImage)
	}
	ogImageURL := g.absoluteMediaURL(canonBase, ogImageID)

	heading := strings.TrimSpace(p.Title)
	if heading == "" {
		heading = htmlTitle
	}

	return pageSEOView{
		Keywords:        keywords,
		Robots:          robots,
		OGTitle:         ogTitle,
		OGDescription:   ogDesc,
		OGType:          ogType,
		OGURL:           canon,
		OGImage:         ogImageURL,
		OGSiteName:      siteName,
		TwitterCard:     "summary_large_image",
		FaviconHTML:     g.faviconLinkHTML(settings),
		PageHeading:     heading,
		ShowPageHeading: heading != "",
	}
}

func (g *Generator) absoluteMediaURL(canonBase, mediaIDOrURL string) string {
	mediaIDOrURL = strings.TrimSpace(mediaIDOrURL)
	if mediaIDOrURL == "" {
		return ""
	}
	if strings.HasPrefix(mediaIDOrURL, "http://") || strings.HasPrefix(mediaIDOrURL, "https://") {
		return mediaIDOrURL
	}
	rel := ""
	if strings.HasPrefix(mediaIDOrURL, "/") {
		rel = mediaIDOrURL
	} else if m, err := g.Store.GetMedia(mediaIDOrURL); err == nil {
		if m.Filename == "" {
			return ""
		}
		_ = g.copyMedia(m)
		rel = "/assets/cdn/" + m.Filename
	} else {
		_ = g.copyMedia(cms.Media{ID: mediaIDOrURL, Filename: mediaIDOrURL, URL: "/media/" + mediaIDOrURL})
		rel = "/assets/cdn/" + mediaIDOrURL
	}
	return strings.TrimRight(canonBase, "/") + pathURL(rel)
}

func (g *Generator) faviconLinkHTML(settings cms.SiteSettings) template.HTML {
	var b strings.Builder
	emit := func(rel, href, typ, sizes string) {
		if href == "" {
			return
		}
		b.WriteString(`<link rel="`)
		b.WriteString(template.HTMLEscapeString(rel))
		b.WriteString(`" href="`)
		b.WriteString(template.HTMLEscapeString(href))
		b.WriteString(`"`)
		if typ != "" {
			b.WriteString(` type="`)
			b.WriteString(template.HTMLEscapeString(typ))
			b.WriteString(`"`)
		}
		if sizes != "" {
			b.WriteString(` sizes="`)
			b.WriteString(template.HTMLEscapeString(sizes))
			b.WriteString(`"`)
		}
		b.WriteString("/>\n")
	}

	customID := strings.TrimSpace(settings.FaviconMediaID)
	if customID != "" {
		if m, err := g.Store.GetMedia(customID); err == nil && m.Filename != "" {
			_ = g.copyMedia(m)
			href := pathURL("/assets/cdn/" + m.Filename)
			ext := strings.ToLower(filepath.Ext(m.Filename))
			typ := m.Mime
			if typ == "" {
				switch ext {
				case ".svg":
					typ = "image/svg+xml"
				case ".png":
					typ = "image/png"
				case ".ico":
					typ = "image/x-icon"
				case ".webp":
					typ = "image/webp"
				}
			}
			emit("icon", href, typ, "32x32")
			emit("apple-touch-icon", href, "", "180x180")
		}
	}

	for _, l := range defaultFaviconLinks {
		emit(l.Rel, l.Href, l.Type, l.Sizes)
	}
	return template.HTML(b.String())
}

func normalizeCanonicalURL(raw, canonBase string) string {
	raw = strings.TrimSpace(raw)
	canonBase = strings.TrimRight(canonBase, "/")
	if raw == "" {
		return canonBase + "/"
	}
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		if strings.TrimRight(raw, "/") == canonBase {
			return canonBase + "/"
		}
		base := filepath.Base(raw)
		if strings.Contains(base, ".") && !strings.HasSuffix(raw, "/") {
			return raw
		}
		if !strings.HasSuffix(raw, "/") {
			return raw + "/"
		}
		return raw
	}
	if !strings.HasPrefix(raw, "/") {
		raw = "/" + raw
	}
	if raw == "/" {
		return canonBase + "/"
	}
	return canonBase + strings.TrimRight(raw, "/") + "/"
}

func pagePublicPath(p cms.Page) string {
	if p.IsHomepage {
		return "/"
	}
	slug := strings.Trim(p.Slug, "/")
	if slug == "" {
		return "/"
	}
	return "/" + slug + "/"
}

func pageCanonicalPath(p cms.Page) string {
	path := strings.TrimSpace(p.CanonicalPath)
	if path == "" && p.SEO != nil {
		path = strings.TrimSpace(p.SEO.CanonicalPath)
	}
	if path != "" {
		return path
	}
	return pagePublicPath(p)
}

func (g *Generator) writeRobotsAndSitemap(pages []cms.Page, settings cms.SiteSettings) error {
	canonBase := settings.CanonicalBase
	if g.Cfg.CanonicalBase != "" {
		canonBase = g.Cfg.CanonicalBase
	}
	canonBase = strings.TrimRight(canonBase, "/")
	if canonBase == "" {
		canonBase = "https://sheyanova.art"
	}

	robots := fmt.Sprintf("User-agent: *\nAllow: /\n\nSitemap: %s/sitemap.xml\n", canonBase)
	if err := os.WriteFile(filepath.Join(g.Cfg.OutDir, "robots.txt"), []byte(robots), 0o644); err != nil {
		return err
	}

	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
	for _, p := range pages {
		if g.Cfg.PublishedOnly && !p.IsPublished() {
			continue
		}
		if !g.Cfg.PublishedOnly && !p.IsPublished() {
			// Draft preview may include drafts; still list them for local QA.
		}
		loc := normalizeCanonicalURL(pageCanonicalPath(p), canonBase)
		b.WriteString("  <url><loc>")
		b.WriteString(template.HTMLEscapeString(loc))
		b.WriteString("</loc></url>\n")
	}
	b.WriteString("</urlset>\n")
	return os.WriteFile(filepath.Join(g.Cfg.OutDir, "sitemap.xml"), []byte(b.String()), 0o644)
}
