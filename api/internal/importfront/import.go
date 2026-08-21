package importfront

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"mime"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"sheyanova.art/api/internal/cms"
)

// Result summarizes an ImportFront run.
type Result struct {
	FrontDir       string            `json:"front_dir"`
	Force          bool              `json:"force"`
	MediaCreated   int               `json:"media_created"`
	MediaReused    int               `json:"media_reused"`
	PagesUpdated   int               `json:"pages_updated"`
	PagesSkipped   int               `json:"pages_skipped"`
	PagesMissing   []string          `json:"pages_missing,omitempty"`
	PageBlocks     map[string]int    `json:"page_blocks,omitempty"`
	Warnings       []string          `json:"warnings,omitempty"`
}

// pageSpec maps CMS slug → relative HTML under front/.
var pageSpecs = []struct {
	Slug     string
	HTMLRel  string // preferred path under frontDir
	AltRels  []string
	Kind     string // ba | gallery | contact
}{
	{Slug: "before-after", HTMLRel: "index.html", AltRels: []string{"before-after/index.html"}, Kind: "ba"},
	{Slug: "editorial", HTMLRel: "editorial/index.html", Kind: "gallery"},
	{Slug: "editorial-i", HTMLRel: "editorial-i/index.html", Kind: "gallery"},
	{Slug: "editorial-ii", HTMLRel: "editorial-ii/index.html", Kind: "gallery"},
	{Slug: "editorial-3", HTMLRel: "editorial-3/index.html", Kind: "gallery"},
	{Slug: "editorial-iv", HTMLRel: "editorial-iv/index.html", Kind: "gallery"},
	{Slug: "fashion", HTMLRel: "fashion/index.html", Kind: "gallery"},
	{Slug: "product", HTMLRel: "product/index.html", Kind: "gallery"},
	{Slug: "about", HTMLRel: "about/index.html", Kind: "gallery"},
	{Slug: "contact", HTMLRel: "contact/index.html", Kind: "contact"},
}

// Import fills CMS blocks from a static front/ tree.
// When force is false, pages that already have any blocks are skipped.
func Import(store *cms.Store, frontDir string, force bool) (Result, error) {
	res := Result{
		FrontDir:   frontDir,
		Force:      force,
		PageBlocks: map[string]int{},
	}
	frontDir = strings.TrimSpace(frontDir)
	if frontDir == "" {
		return res, fmt.Errorf("front dir is empty")
	}
	st, err := os.Stat(frontDir)
	if err != nil || !st.IsDir() {
		return res, fmt.Errorf("front dir not found: %s", frontDir)
	}

	pages, err := store.ListPages()
	if err != nil {
		return res, err
	}
	bySlug := map[string]cms.Page{}
	for _, p := range pages {
		bySlug[p.Slug] = p
	}

	mediaIndex, err := buildMediaIndex(store)
	if err != nil {
		return res, err
	}

	ensure := func(assetURL string) (cms.Media, error) {
		m, created, err := ensureMedia(store, frontDir, assetURL, mediaIndex)
		if err != nil {
			return cms.Media{}, err
		}
		if created {
			res.MediaCreated++
		} else {
			res.MediaReused++
		}
		return m, nil
	}

	for _, spec := range pageSpecs {
		page, ok := bySlug[spec.Slug]
		if !ok {
			res.PagesMissing = append(res.PagesMissing, spec.Slug)
			continue
		}

		existing, err := store.ListBlocks(page.ID)
		if err != nil {
			return res, err
		}
		if len(existing) > 0 && !force {
			// Contact is seeded with placeholder blocks; still skip unless force.
			res.PagesSkipped++
			res.PageBlocks[spec.Slug] = len(existing)
			continue
		}

		htmlPath, html, err := readPageHTML(frontDir, spec.HTMLRel, spec.AltRels)
		if err != nil {
			res.Warnings = append(res.Warnings, fmt.Sprintf("%s: %v", spec.Slug, err))
			continue
		}
		_ = htmlPath

		var blocks []cms.Block
		switch spec.Kind {
		case "ba":
			pairs := parseComparisonPairs(html)
			if len(pairs) == 0 {
				res.Warnings = append(res.Warnings, fmt.Sprintf("%s: no comparison sliders found", spec.Slug))
				continue
			}
			for _, pair := range pairs {
				before, err := ensure(pair.BeforeURL)
				if err != nil {
					res.Warnings = append(res.Warnings, fmt.Sprintf("%s before %s: %v", spec.Slug, pair.BeforeURL, err))
					continue
				}
				after, err := ensure(pair.AfterURL)
				if err != nil {
					res.Warnings = append(res.Warnings, fmt.Sprintf("%s after %s: %v", spec.Slug, pair.AfterURL, err))
					continue
				}
				blocks = append(blocks, cms.Block{
					Type: cms.BlockComparisonSlider,
					Data: cms.MustJSON(map[string]any{
						"before_media_id": before.ID,
						"after_media_id":  after.ID,
						"before_url":      before.URL,
						"after_url":       after.URL,
					}),
				})
			}
		case "gallery":
			imgs := parseGalleryImages(html)
			if len(imgs) == 0 {
				res.Warnings = append(res.Warnings, fmt.Sprintf("%s: no gallery images found", spec.Slug))
				continue
			}
			for _, img := range imgs {
				m, err := ensure(img.URL)
				if err != nil {
					res.Warnings = append(res.Warnings, fmt.Sprintf("%s image %s: %v", spec.Slug, img.URL, err))
					continue
				}
				data := map[string]any{
					"media_id": m.ID,
					"url":      m.URL,
				}
				if img.Alt != "" {
					data["alt"] = img.Alt
				}
				blocks = append(blocks, cms.Block{
					Type: cms.BlockGalleryImage,
					Data: cms.MustJSON(data),
				})
			}
		case "contact":
			c := parseContact(html)
			blocks = []cms.Block{
				{Type: cms.BlockRichText, Data: cms.MustJSON(map[string]any{
					"html": fmt.Sprintf(`<h2 class="xl-headline">%s</h2>`, escapeBasic(c.Headline)),
				})},
				{Type: cms.BlockContactForm, Data: cms.MustJSON(map[string]any{
					"name_label":    c.NameLabel,
					"email_label":   c.EmailLabel,
					"message_label": c.MessageLabel,
					"submit_label":  c.SubmitLabel,
				})},
			}
		}

		if len(blocks) == 0 {
			res.Warnings = append(res.Warnings, fmt.Sprintf("%s: nothing to import", spec.Slug))
			continue
		}
		out, err := store.ReplaceBlocks(page.ID, blocks)
		if err != nil {
			return res, fmt.Errorf("replace blocks %s: %w", spec.Slug, err)
		}
		res.PagesUpdated++
		res.PageBlocks[spec.Slug] = len(out)
	}

	return res, nil
}

// HasFrontContent reports whether frontDir looks like the static site to import.
func HasFrontContent(frontDir string) bool {
	if strings.TrimSpace(frontDir) == "" {
		return false
	}
	for _, rel := range []string{"index.html", "before-after/index.html", "editorial/index.html"} {
		if st, err := os.Stat(filepath.Join(frontDir, rel)); err == nil && !st.IsDir() {
			return true
		}
	}
	return false
}

func readPageHTML(frontDir, primary string, alts []string) (string, string, error) {
	candidates := append([]string{primary}, alts...)
	var lastErr error
	for _, rel := range candidates {
		path := filepath.Join(frontDir, filepath.FromSlash(rel))
		b, err := os.ReadFile(path)
		if err != nil {
			lastErr = err
			continue
		}
		return path, string(b), nil
	}
	if lastErr == nil {
		lastErr = os.ErrNotExist
	}
	return "", "", lastErr
}

type mediaIndex struct {
	byID       map[string]cms.Media
	byFilename map[string]cms.Media
	byOriginal map[string]cms.Media
}

func buildMediaIndex(store *cms.Store) (*mediaIndex, error) {
	list, err := store.ListMedia()
	if err != nil {
		return nil, err
	}
	idx := &mediaIndex{
		byID:       map[string]cms.Media{},
		byFilename: map[string]cms.Media{},
		byOriginal: map[string]cms.Media{},
	}
	for _, m := range list {
		idx.byID[m.ID] = m
		if m.Filename != "" {
			idx.byFilename[strings.ToLower(m.Filename)] = m
		}
		if m.OriginalName != "" {
			idx.byOriginal[strings.ToLower(m.OriginalName)] = m
		}
	}
	return idx, nil
}

func (idx *mediaIndex) remember(m cms.Media) {
	idx.byID[m.ID] = m
	if m.Filename != "" {
		idx.byFilename[strings.ToLower(m.Filename)] = m
	}
	if m.OriginalName != "" {
		idx.byOriginal[strings.ToLower(m.OriginalName)] = m
	}
}

func ensureMedia(store *cms.Store, frontDir, assetURL string, idx *mediaIndex) (cms.Media, bool, error) {
	assetURL = strings.TrimSpace(assetURL)
	if assetURL == "" {
		return cms.Media{}, false, fmt.Errorf("empty asset url")
	}
	relWeb := assetURL
	if strings.HasPrefix(relWeb, "http://") || strings.HasPrefix(relWeb, "https://") {
		return cms.Media{}, false, fmt.Errorf("remote asset not supported: %s", assetURL)
	}
	relWeb = strings.TrimPrefix(relWeb, "/")
	decoded, err := url.PathUnescape(relWeb)
	if err != nil {
		decoded = relWeb
	}

	basename := filepath.Base(decoded)
	stableID := stableMediaID(decoded)

	if m, ok := idx.byID[stableID]; ok {
		if err := ensureUploadCopy(store, frontDir, decoded, m.Filename); err != nil {
			log.Printf("importfront: copy warning %s: %v", decoded, err)
		}
		return m, false, nil
	}
	if m, ok := idx.byOriginal[strings.ToLower(basename)]; ok {
		if err := ensureUploadCopy(store, frontDir, decoded, m.Filename); err != nil {
			log.Printf("importfront: copy warning %s: %v", decoded, err)
		}
		return m, false, nil
	}
	if m, ok := idx.byFilename[strings.ToLower(basename)]; ok {
		if err := ensureUploadCopy(store, frontDir, decoded, m.Filename); err != nil {
			log.Printf("importfront: copy warning %s: %v", decoded, err)
		}
		return m, false, nil
	}

	src, err := resolveFrontFile(frontDir, decoded, relWeb)
	if err != nil {
		return cms.Media{}, false, err
	}
	fi, err := os.Stat(src)
	if err != nil {
		return cms.Media{}, false, err
	}

	ext := strings.ToLower(filepath.Ext(basename))
	uploadName := basename
	dst := filepath.Join(store.UploadDir(), uploadName)
	// Avoid clobbering a different file with the same basename.
	if st, err := os.Stat(dst); err == nil && st.Size() != fi.Size() {
		uploadName = stableID + ext
		dst = filepath.Join(store.UploadDir(), uploadName)
	}
	if err := copyFile(src, dst); err != nil {
		return cms.Media{}, false, err
	}

	mimeType := mime.TypeByExtension(ext)
	m := cms.Media{
		ID:           stableID,
		Filename:     uploadName,
		OriginalName: basename,
		URL:          "/media/" + uploadName,
		Title:        strings.TrimSuffix(basename, filepath.Ext(basename)),
		Kind:         "image",
		Mime:         mimeType,
		SizeBytes:    fi.Size(),
	}
	out, err := store.CreateMedia(m)
	if err != nil {
		// Race / already exists: try fetch by id.
		if existing, gerr := store.GetMedia(stableID); gerr == nil {
			idx.remember(existing)
			return existing, false, nil
		}
		return cms.Media{}, false, err
	}
	idx.remember(out)
	return out, true, nil
}

func resolveFrontFile(frontDir, decodedRel, rawRel string) (string, error) {
	candidates := []string{
		filepath.Join(frontDir, filepath.FromSlash(decodedRel)),
		filepath.Join(frontDir, filepath.FromSlash(rawRel)),
	}
	// Also try basename under assets/cdn in case of path quirks.
	base := filepath.Base(decodedRel)
	candidates = append(candidates, filepath.Join(frontDir, "assets", "cdn", base))

	var lastErr error
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c, nil
		} else if err != nil {
			lastErr = err
		}
	}
	if lastErr == nil {
		lastErr = os.ErrNotExist
	}
	return "", fmt.Errorf("file not found for %s: %w", decodedRel, lastErr)
}

func ensureUploadCopy(store *cms.Store, frontDir, decodedRel, uploadFilename string) error {
	if uploadFilename == "" {
		return nil
	}
	dst := filepath.Join(store.UploadDir(), uploadFilename)
	if st, err := os.Stat(dst); err == nil && st.Size() > 0 {
		return nil
	}
	src, err := resolveFrontFile(frontDir, decodedRel, decodedRel)
	if err != nil {
		return err
	}
	return copyFile(src, dst)
}

func stableMediaID(decodedRel string) string {
	sum := sha256.Sum256([]byte("front:" + strings.ToLower(filepath.ToSlash(decodedRel))))
	return hex.EncodeToString(sum[:8])
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func escapeBasic(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
	)
	return r.Replace(s)
}
