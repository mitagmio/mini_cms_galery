package generate

import (
	_ "embed"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"

	"sheyanova.art/api/internal/cms"
)

//go:embed assets/about.css
var articleCSS []byte

func aboutHeroClass(hasPhoto, hasBio bool) string {
	c := "about-hero"
	if hasPhoto && hasBio {
		return c
	}
	c += " about-hero--solo"
	if hasPhoto {
		c += " about-hero--photo-only"
	}
	if hasBio {
		c += " about-hero--text-only"
	}
	return c
}

func splitAboutHero(blocks []renderedBlock) (portrait, bio template.HTML, rest []renderedBlock) {
	rest = make([]renderedBlock, 0)
	var gotPhoto, gotBio bool
	for _, b := range blocks {
		if !gotPhoto && b.Type == cms.BlockGalleryImage {
			portrait = b.HTML
			gotPhoto = true
			continue
		}
		if !gotBio && b.Type == cms.BlockRichText {
			bio = b.HTML
			gotBio = true
			continue
		}
		rest = append(rest, b)
	}
	return portrait, bio, rest
}

func articleFigureHTML(src, alt, caption, dim, loading, extraClass string) string {
	capHTML := ""
	if strings.TrimSpace(caption) != "" {
		capHTML = fmt.Sprintf(`<figcaption class="about-caption">%s</figcaption>`, template.HTMLEscapeString(caption))
	}
	cls := "article-figure about-figure"
	if extraClass != "" {
		cls += " " + extraClass
	}
	if loading == "" {
		loading = "lazy"
	}
	return fmt.Sprintf(`
<figure class="%s">
<img alt="%s" class="about-photo" decoding="async" loading="%s"%s src="%s"/>
%s
</figure>`, cls, template.HTMLEscapeString(alt), template.HTMLEscapeString(loading), dim, src, capHTML)
}

func (g *Generator) writeArticleCSS() error {
	if len(articleCSS) == 0 {
		return nil
	}
	// Draft OutDir is /data/preview; ThemeSrc is /front. Only write into OutDir so
	// GeneratePage/GeneratePreview never mutate the live theme tree. Publish sets
	// OutDir == ThemeSrc (/front), so about.css still lands in the published site.
	p := filepath.Join(g.Cfg.OutDir, "assets", "theme", "about.css")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, articleCSS, 0o644)
}
