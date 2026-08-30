package cms

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	_ "golang.org/x/image/webp"
)

func TestGenerateMediaBannerResizes(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "big.png")
	f, err := os.Create(src)
	if err != nil {
		t.Fatal(err)
	}
	img := image.NewNRGBA(image.Rect(0, 0, 1500, 2000))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.NRGBA{R: 180, G: 160, B: 140, A: 255}}, image.Point{}, draw.Src)
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	fn, err := GenerateMediaBanner(dir, "abc123", "big.png")
	if err != nil {
		t.Fatal(err)
	}
	if fn != "abc123_banner.webp" {
		t.Fatalf("fn=%s", fn)
	}
	got, err := decodeImage(filepath.Join(dir, fn))
	if err != nil {
		t.Fatal(err)
	}
	b := got.Bounds()
	if b.Dy() != bannerMaxSide {
		t.Fatalf("longest side want %d got %dx%d", bannerMaxSide, b.Dx(), b.Dy())
	}
	again, err := EnsureMediaBanner(dir, Media{ID: "abc123", Filename: "big.png"})
	if err != nil || again != fn {
		t.Fatalf("ensure=%s err=%v", again, err)
	}
}

func TestGenerateMediaDisplayResizes(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "huge.png")
	f, err := os.Create(src)
	if err != nil {
		t.Fatal(err)
	}
	img := image.NewNRGBA(image.Rect(0, 0, 4000, 6000))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.NRGBA{R: 100, G: 120, B: 140, A: 255}}, image.Point{}, draw.Src)
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	fn, err := GenerateMediaDisplay(dir, "disp01", "huge.png")
	if err != nil {
		t.Fatal(err)
	}
	if fn != "disp01_display.webp" {
		t.Fatalf("fn=%s", fn)
	}
	got, err := decodeImage(filepath.Join(dir, fn))
	if err != nil {
		t.Fatal(err)
	}
	b := got.Bounds()
	if b.Dy() != displayMaxSide {
		t.Fatalf("longest side want %d got %dx%d", displayMaxSide, b.Dx(), b.Dy())
	}
	again, err := EnsureMediaDisplay(dir, Media{ID: "disp01", Filename: "huge.png"})
	if err != nil || again != fn {
		t.Fatalf("ensure=%s err=%v", again, err)
	}
}

func decodeImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	return img, err
}
