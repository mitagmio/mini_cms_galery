package cms

import (
	"fmt"
	"image"
	"image/jpeg"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/HugoSmits86/nativewebp"
	"github.com/disintegration/imaging"
	_ "golang.org/x/image/webp" // register WebP decoder
)

const (
	thumbMaxSide = 400
	// bannerMaxSide fits rates 3-col tiles (max grid ~960px → ~308px CSS, ~2× retina).
	bannerMaxSide = 1000
	// displayMaxSide for panorama / lookbook / BA site galleries (~2× retina up to ~1000 CSS px).
	displayMaxSide = 2000
	thumbJPEGQual  = 78
)

// GenerateMediaThumb resizes the original to max longest side thumbMaxSide and
// writes {id}_thumb.webp (or .jpg fallback). SVG is skipped (empty names, nil error).
func GenerateMediaThumb(uploadDir, id, srcFilename string) (thumbFilename, thumbURL string, err error) {
	fn, err := generateMediaVariant(uploadDir, id, srcFilename, "_thumb", thumbMaxSide)
	if err != nil || fn == "" {
		return "", "", err
	}
	return fn, "/media/" + fn, nil
}

// GenerateMediaBanner writes {id}_banner.webp (~bannerMaxSide px) for rates tiles.
// SVG is skipped (empty name, nil error). Falls back to .jpg if WebP encode fails.
func GenerateMediaBanner(uploadDir, id, srcFilename string) (bannerFilename string, err error) {
	return generateMediaVariant(uploadDir, id, srcFilename, "_banner", bannerMaxSide)
}

// GenerateMediaDisplay writes {id}_display.webp (~displayMaxSide px) for galleries/BA.
// SVG is skipped (empty name, nil error). Falls back to .jpg if WebP encode fails.
func GenerateMediaDisplay(uploadDir, id, srcFilename string) (displayFilename string, err error) {
	return generateMediaVariant(uploadDir, id, srcFilename, "_display", displayMaxSide)
}

// EnsureMediaBanner returns an existing {id}_banner.* or generates one.
// Falls back to empty filename if the source is SVG / unreadable.
func EnsureMediaBanner(uploadDir string, m Media) (string, error) {
	return ensureMediaVariant(uploadDir, m, "_banner", bannerMaxSide)
}

// EnsureMediaDisplay returns an existing {id}_display.* or generates one.
// Falls back to empty filename if the source is SVG / unreadable.
func EnsureMediaDisplay(uploadDir string, m Media) (string, error) {
	return ensureMediaVariant(uploadDir, m, "_display", displayMaxSide)
}

func ensureMediaVariant(uploadDir string, m Media, suffix string, maxSide int) (string, error) {
	id := strings.TrimSpace(m.ID)
	if id == "" {
		return "", nil
	}
	for _, name := range []string{id + suffix + ".webp", id + suffix + ".jpg"} {
		p := filepath.Join(uploadDir, name)
		if st, err := os.Stat(p); err == nil && !st.IsDir() && st.Size() > 0 {
			return name, nil
		}
	}
	return generateMediaVariant(uploadDir, id, m.Filename, suffix, maxSide)
}

func generateMediaVariant(uploadDir, id, srcFilename, suffix string, maxSide int) (filename string, err error) {
	ext := strings.ToLower(filepath.Ext(srcFilename))
	if ext == ".svg" {
		return "", nil
	}
	srcPath := filepath.Join(uploadDir, srcFilename)
	img, err := imaging.Open(srcPath, imaging.AutoOrientation(true))
	if err != nil {
		return "", fmt.Errorf("open: %w", err)
	}
	img = resizeLongestSide(img, maxSide)

	webpName := id + suffix + ".webp"
	webpPath := filepath.Join(uploadDir, webpName)
	if err := encodeWebP(webpPath, img); err == nil {
		return webpName, nil
	}

	jpgName := id + suffix + ".jpg"
	jpgPath := filepath.Join(uploadDir, jpgName)
	if err := encodeJPEG(jpgPath, img, thumbJPEGQual); err != nil {
		return "", fmt.Errorf("encode jpeg: %w", err)
	}
	_ = os.Remove(webpPath) // clean failed/partial webp if any
	return jpgName, nil
}

func resizeLongestSide(img image.Image, maxSide int) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return img
	}
	if w <= maxSide && h <= maxSide {
		return img
	}
	if w >= h {
		return imaging.Resize(img, maxSide, 0, imaging.Lanczos)
	}
	return imaging.Resize(img, 0, maxSide, imaging.Lanczos)
}

func encodeWebP(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return nativewebp.Encode(f, img, nil)
}

func encodeJPEG(path string, img image.Image, quality int) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return jpeg.Encode(f, img, &jpeg.Options{Quality: quality})
}

func removeMediaThumbFiles(uploadDir string, m Media) {
	if m.ThumbFilename != "" {
		_ = os.Remove(filepath.Join(uploadDir, m.ThumbFilename))
	}
	// Best-effort cleanup of either naming variant.
	for _, suffix := range []string{"_thumb", "_banner", "_display"} {
		_ = os.Remove(filepath.Join(uploadDir, m.ID+suffix+".webp"))
		_ = os.Remove(filepath.Join(uploadDir, m.ID+suffix+".jpg"))
	}
}

// VariantBackfillStats counts outcomes of EnsureMediaVariants.
type VariantBackfillStats struct {
	ThumbCreated   int `json:"thumb_created"`
	BannerCreated  int `json:"banner_created"`
	DisplayCreated int `json:"display_created"`
	Skipped        int `json:"skipped"`
	Failed         int `json:"failed"`
}

// EnsureMediaThumbs generates missing thumbs for all media rows. Safe to call on boot.
// Prefer EnsureMediaVariants when banner/display backfill is also needed.
func (s *Store) EnsureMediaThumbs() (created, skipped, failed int, err error) {
	st, err := s.EnsureMediaVariants()
	return st.ThumbCreated, st.Skipped, st.Failed, err
}

// EnsureMediaVariants generates missing thumb + banner + display for every media row.
func (s *Store) EnsureMediaVariants() (VariantBackfillStats, error) {
	var st VariantBackfillStats
	list, err := s.ListMedia()
	if err != nil {
		return st, err
	}
	for _, m := range list {
		ext := strings.ToLower(filepath.Ext(m.Filename))
		if ext == ".svg" {
			st.Skipped++
			continue
		}
		src := filepath.Join(s.uploadDir, m.Filename)
		if _, e := os.Stat(src); e != nil {
			st.Failed++
			log.Printf("cms: variant skip missing original id=%s", m.ID)
			continue
		}

		rowOK := true
		thumbExisted := mediaVariantExists(s.uploadDir, m.ID, "_thumb", m.ThumbFilename)
		bannerExisted := mediaVariantExists(s.uploadDir, m.ID, "_banner", "")
		displayExisted := mediaVariantExists(s.uploadDir, m.ID, "_display", "")

		if !thumbExisted {
			fn, url, gerr := GenerateMediaThumb(s.uploadDir, m.ID, m.Filename)
			if gerr != nil {
				st.Failed++
				rowOK = false
				log.Printf("cms: thumb generate id=%s: %v", m.ID, gerr)
			} else if fn == "" {
				// unreadable / skipped encode
			} else if err := s.SetMediaThumb(m.ID, fn, url); err != nil {
				st.Failed++
				rowOK = false
				log.Printf("cms: thumb save id=%s: %v", m.ID, err)
			} else {
				st.ThumbCreated++
			}
		}

		if !bannerExisted {
			fn, gerr := GenerateMediaBanner(s.uploadDir, m.ID, m.Filename)
			if gerr != nil {
				st.Failed++
				rowOK = false
				log.Printf("cms: banner generate id=%s: %v", m.ID, gerr)
			} else if fn != "" {
				st.BannerCreated++
			}
		}

		if !displayExisted {
			fn, gerr := GenerateMediaDisplay(s.uploadDir, m.ID, m.Filename)
			if gerr != nil {
				st.Failed++
				rowOK = false
				log.Printf("cms: display generate id=%s: %v", m.ID, gerr)
			} else if fn != "" {
				st.DisplayCreated++
			}
		}

		if thumbExisted && bannerExisted && displayExisted {
			st.Skipped++
		} else if !rowOK {
			// already counted in Failed
		}
	}
	return st, nil
}

func mediaVariantExists(uploadDir, id, suffix, knownFilename string) bool {
	if knownFilename != "" {
		p := filepath.Join(uploadDir, knownFilename)
		if st, err := os.Stat(p); err == nil && !st.IsDir() && st.Size() > 0 {
			return true
		}
	}
	for _, name := range []string{id + suffix + ".webp", id + suffix + ".jpg"} {
		p := filepath.Join(uploadDir, name)
		if st, err := os.Stat(p); err == nil && !st.IsDir() && st.Size() > 0 {
			return true
		}
	}
	return false
}

// SetMediaThumb updates thumb_filename / thumb_url for a media row.
func (s *Store) SetMediaThumb(id, filename, url string) error {
	_, err := s.db.Exec(`UPDATE media SET thumb_filename=?, thumb_url=?, updated_at=? WHERE id=?`,
		filename, url, Now(), id)
	return err
}
