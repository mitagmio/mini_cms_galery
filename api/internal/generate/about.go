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
	seen := map[string]bool{}
	paths := []string{filepath.Join(g.Cfg.OutDir, "assets", "theme", "about.css")}
	if src := strings.TrimSpace(g.Cfg.ThemeSrc); src != "" {
		paths = append(paths, filepath.Join(src, "assets", "theme", "about.css"))
	}
	for _, p := range paths {
		abs, err := filepath.Abs(p)
		if err != nil {
			abs = p
		}
		if seen[abs] {
			continue
		}
		seen[abs] = true
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(p, articleCSS, 0o644); err != nil {
			return err
		}
	}
	return nil
}
