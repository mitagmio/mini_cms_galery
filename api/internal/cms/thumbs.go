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
	thumbMaxSide  = 400
	thumbJPEGQual = 78
)

// GenerateMediaThumb resizes the original to max longest side thumbMaxSide and
// writes {id}_thumb.webp (or .jpg fallback). SVG is skipped (empty names, nil error).
func GenerateMediaThumb(uploadDir, id, srcFilename string) (thumbFilename, thumbURL string, err error) {
	ext := strings.ToLower(filepath.Ext(srcFilename))
	if ext == ".svg" {
		return "", "", nil
	}
	srcPath := filepath.Join(uploadDir, srcFilename)
	img, err := imaging.Open(srcPath, imaging.AutoOrientation(true))
	if err != nil {
		return "", "", fmt.Errorf("open: %w", err)
	}
	img = resizeLongestSide(img, thumbMaxSide)

	webpName := id + "_thumb.webp"
	webpPath := filepath.Join(uploadDir, webpName)
	if err := encodeWebP(webpPath, img); err == nil {
		return webpName, "/media/" + webpName, nil
	}

	jpgName := id + "_thumb.jpg"
	jpgPath := filepath.Join(uploadDir, jpgName)
	if err := encodeJPEG(jpgPath, img, thumbJPEGQual); err != nil {
		return "", "", fmt.Errorf("encode jpeg: %w", err)
	}
	_ = os.Remove(webpPath) // clean failed/partial webp if any
	return jpgName, "/media/" + jpgName, nil
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
	_ = os.Remove(filepath.Join(uploadDir, m.ID+"_thumb.webp"))
	_ = os.Remove(filepath.Join(uploadDir, m.ID+"_thumb.jpg"))
}

// EnsureMediaThumbs generates missing thumbs for all media rows. Safe to call on boot.
func (s *Store) EnsureMediaThumbs() (created, skipped, failed int, err error) {
	list, err := s.ListMedia()
	if err != nil {
		return 0, 0, 0, err
	}
	for _, m := range list {
		if m.ThumbFilename != "" {
			p := filepath.Join(s.uploadDir, m.ThumbFilename)
			if st, e := os.Stat(p); e == nil && !st.IsDir() && st.Size() > 0 {
				skipped++
				continue
			}
		}
		ext := strings.ToLower(filepath.Ext(m.Filename))
		if ext == ".svg" {
			skipped++
			continue
		}
		src := filepath.Join(s.uploadDir, m.Filename)
		if _, e := os.Stat(src); e != nil {
			failed++
			log.Printf("cms: thumb skip missing original id=%s", m.ID)
			continue
		}
		fn, url, gerr := GenerateMediaThumb(s.uploadDir, m.ID, m.Filename)
		if gerr != nil {
			failed++
			log.Printf("cms: thumb generate id=%s: %v", m.ID, gerr)
			continue
		}
		if fn == "" {
			skipped++
			continue
		}
		if err := s.SetMediaThumb(m.ID, fn, url); err != nil {
			failed++
			log.Printf("cms: thumb save id=%s: %v", m.ID, err)
			continue
		}
		created++
	}
	return created, skipped, failed, nil
}

// SetMediaThumb updates thumb_filename / thumb_url for a media row.
func (s *Store) SetMediaThumb(id, filename, url string) error {
	_, err := s.db.Exec(`UPDATE media SET thumb_filename=?, thumb_url=?, updated_at=? WHERE id=?`,
		filename, url, Now(), id)
	return err
}
