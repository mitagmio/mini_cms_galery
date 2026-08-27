package generate

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/disintegration/imaging"
	_ "golang.org/x/image/webp" // register WebP decoder for thumbs
)

const (
	rateLumLightThreshold = 0.55
	// rateTextOutline is stored in text_color for white fill + black stroke.
	rateTextOutline = "outline"
	// rateTextCharcoal matches Beauty placeholder / --light solid near-black.
	rateTextCharcoal = "#1a1a1a"
)

var hexColorRe = regexp.MustCompile(`(?i)^#([0-9a-f]{3}|[0-9a-f]{6})$`)

type lumSample struct {
	v  float64
	ok bool
}

// NormalizeRateTextColor returns a lowercase #rrggbb hex, "outline" for the
// white+stroke preset, or "" for Auto.
func NormalizeRateTextColor(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" || s == "auto" {
		return ""
	}
	if s == rateTextOutline || s == "white_stroke" {
		return rateTextOutline
	}
	// Legacy coral label / Format-ish coral → neutral custom navy (not a preset).
	if s == "coral" || s == "#c07359" || s == "c07359" {
		return "#1a4a7a"
	}
	if !strings.HasPrefix(s, "#") {
		s = "#" + s
	}
	if !hexColorRe.MatchString(s) {
		return ""
	}
	if len(s) == 4 {
		s = fmt.Sprintf("#%c%c%c%c%c%c", s[1], s[1], s[2], s[2], s[3], s[3])
	}
	// Legacy charcoal hex → Beauty charcoal.
	if s == "#2f2f2f" {
		return rateTextCharcoal
	}
	return s
}

func parseHexRGB(hex string) (r, g, b float64, ok bool) {
	hex = NormalizeRateTextColor(hex)
	if hex == "" || hex == rateTextOutline || !strings.HasPrefix(hex, "#") {
		return 0, 0, 0, false
	}
	n, err := strconv.ParseUint(hex[1:], 16, 32)
	if err != nil {
		return 0, 0, 0, false
	}
	return float64(n>>16) / 255, float64((n>>8)&0xff) / 255, float64(n&0xff) / 255, true
}

func relativeLuminanceRGB(r, g, b float64) float64 {
	return 0.2126*r + 0.7152*g + 0.0722*b
}

func isLightHex(hex string) bool {
	r, g, b, ok := parseHexRGB(hex)
	if !ok {
		return false
	}
	return relativeLuminanceRGB(r, g, b) >= rateLumLightThreshold
}

// sampleBottomLuminance averages relative luminance over the bottom ~30% of the image
// (where the rate overlay sits). Large images are shrunk first for speed.
func sampleBottomLuminance(path string) (float64, bool) {
	if strings.TrimSpace(path) == "" {
		return 0, false
	}
	img, err := imaging.Open(path, imaging.AutoOrientation(true))
	if err != nil {
		return 0, false
	}
	bounds := img.Bounds()
	if bounds.Dx() < 1 || bounds.Dy() < 1 {
		return 0, false
	}
	if bounds.Dx() > 96 {
		img = imaging.Resize(img, 96, 0, imaging.Box)
		bounds = img.Bounds()
	}
	y0 := bounds.Min.Y + (bounds.Dy()*70)/100
	if y0 >= bounds.Max.Y {
		y0 = bounds.Min.Y
	}
	var sum, n float64
	for y := y0; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			rr, gg, bb, aa := img.At(x, y).RGBA()
			if aa < 0x8000 {
				continue
			}
			sum += relativeLuminanceRGB(float64(rr)/65535, float64(gg)/65535, float64(bb)/65535)
			n++
		}
	}
	if n < 1 {
		return 0, false
	}
	return sum / n, true
}

func (g *Generator) cachedBottomLuminance(path string) (float64, bool) {
	if path == "" {
		return 0, false
	}
	if g.imgLum == nil {
		g.imgLum = make(map[string]lumSample)
	}
	if s, ok := g.imgLum[path]; ok {
		return s.v, s.ok
	}
	v, ok := sampleBottomLuminance(path)
	g.imgLum[path] = lumSample{v: v, ok: ok}
	return v, ok
}

// rateBannerAnalyzePath prefers a media thumb (fast) then the original file.
func (g *Generator) rateBannerAnalyzePath(mediaID, url string) string {
	mediaID = strings.TrimSpace(mediaID)
	if mediaID != "" && g.Store != nil {
		if m, err := g.Store.GetMedia(mediaID); err == nil {
			if fn := strings.TrimSpace(m.ThumbFilename); fn != "" {
				p := filepath.Join(g.Cfg.UploadDir, fn)
				if st, err := os.Stat(p); err == nil && !st.IsDir() {
					return p
				}
			}
		}
	}
	return g.mediaFilePath(mediaID, url)
}

// resolveRateOverlayTone picks CSS class / inline style for overlay text.
// Empty text_color → Auto (binary): light photos → solid charcoal (#1a1a1a /
// rate-banner--light), dark photos → white (rate-banner--dark). Outline is
// never chosen by Auto; placeholders (no image) keep charcoal via .is-placeholder.
// text_color "outline" is a manual preset only; hex values force flat custom color.
func (g *Generator) resolveRateOverlayTone(textColor, analyzePath string, hasImage bool) (extraClass, overlayStyle string) {
	token := NormalizeRateTextColor(textColor)
	if token == rateTextOutline {
		return " rate-banner--outline", ""
	}
	// Charcoal / Beauty-like: solid #1a1a1a via --light (no outline, no wash).
	if token == rateTextCharcoal {
		return " rate-banner--light", ""
	}
	if token != "" {
		shadow := "0 1px 2px rgba(0,0,0,.35)"
		if !isLightHex(token) {
			shadow = "0 1px 2px rgba(255,255,255,.35)"
		}
		// --rate-text drives child -webkit-text-fill-color so custom hex is not lost.
		return " rate-banner--custom", fmt.Sprintf(
			` style="--rate-text:%s;color:%s;text-shadow:%s"`, token, token, shadow,
		)
	}
	if !hasImage {
		return "", ""
	}
	lum, ok := g.cachedBottomLuminance(analyzePath)
	if ok && lum >= rateLumLightThreshold {
		return " rate-banner--light", ""
	}
	return " rate-banner--dark", ""
}