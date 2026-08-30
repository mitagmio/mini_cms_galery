package generate

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSolidPNG(t *testing.T, path string, c color.NRGBA, w, h int) {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, c)
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func TestNormalizeRateTextColor(t *testing.T) {
	cases := map[string]string{
		"":             "",
		"auto":         "",
		"Auto":         "",
		"#fff":         "#ffffff",
		"#2f2f2f":      "#1a1a1a", // legacy charcoal → Beauty charcoal
		"2F2F2F":       "#1a1a1a",
		"#1a1a1a":      "#1a1a1a",
		"coral":        "#1a4a7a",
		"#c07359":      "#1a4a7a",
		"nope":         "",
		"#gg0000":      "",
		"outline":      "outline",
		"Outline":      "outline",
		"white_stroke": "outline",
	}
	for in, want := range cases {
		if got := NormalizeRateTextColor(in); got != want {
			t.Fatalf("NormalizeRateTextColor(%q)=%q want %q", in, got, want)
		}
	}
}

func TestSampleBottomLuminanceLightAndDark(t *testing.T) {
	dir := t.TempDir()
	lightPath := filepath.Join(dir, "light.png")
	darkPath := filepath.Join(dir, "dark.png")
	writeSolidPNG(t, lightPath, color.NRGBA{R: 240, G: 230, B: 210, A: 255}, 40, 60)
	writeSolidPNG(t, darkPath, color.NRGBA{R: 20, G: 24, B: 30, A: 255}, 40, 60)

	lightLum, ok := sampleBottomLuminance(lightPath)
	if !ok || lightLum < rateLumLightThreshold {
		t.Fatalf("light image lum=%v ok=%v", lightLum, ok)
	}
	darkLum, ok := sampleBottomLuminance(darkPath)
	if !ok || darkLum >= rateLumLightThreshold {
		t.Fatalf("dark image lum=%v ok=%v", darkLum, ok)
	}
}

func TestResolveRateOverlayTone(t *testing.T) {
	dir := t.TempDir()
	lightPath := filepath.Join(dir, "light.png")
	darkPath := filepath.Join(dir, "dark.png")
	writeSolidPNG(t, lightPath, color.NRGBA{R: 250, G: 245, B: 230, A: 255}, 32, 48)
	writeSolidPNG(t, darkPath, color.NRGBA{R: 10, G: 12, B: 18, A: 255}, 32, 48)

	g := &Generator{}

	cls, style := g.resolveRateOverlayTone("", lightPath, true)
	if !strings.Contains(cls, "rate-banner--light") || style != "" {
		t.Fatalf("auto light: cls=%q style=%q", cls, style)
	}
	if strings.Contains(cls, "rate-banner--outline") {
		t.Fatalf("auto must not pick outline: cls=%q", cls)
	}
	cls, style = g.resolveRateOverlayTone("", darkPath, true)
	if !strings.Contains(cls, "rate-banner--dark") || style != "" {
		t.Fatalf("auto dark: cls=%q style=%q", cls, style)
	}
	cls, style = g.resolveRateOverlayTone("", "", false)
	if cls != "" || style != "" {
		t.Fatalf("placeholder: cls=%q style=%q", cls, style)
	}
	cls, style = g.resolveRateOverlayTone("outline", darkPath, true)
	if !strings.Contains(cls, "rate-banner--outline") || style != "" {
		t.Fatalf("manual outline: cls=%q style=%q", cls, style)
	}
	cls, style = g.resolveRateOverlayTone("#1a1a1a", lightPath, true)
	if !strings.Contains(cls, "rate-banner--light") || style != "" {
		t.Fatalf("charcoal: cls=%q style=%q", cls, style)
	}
	cls, style = g.resolveRateOverlayTone("#2f2f2f", lightPath, true)
	if !strings.Contains(cls, "rate-banner--light") || style != "" {
		t.Fatalf("legacy charcoal: cls=%q style=%q", cls, style)
	}
	cls, style = g.resolveRateOverlayTone("#ffffff", darkPath, true)
	if !strings.Contains(cls, "rate-banner--custom") || !strings.Contains(style, "color:#ffffff") {
		t.Fatalf("manual white: cls=%q style=%q", cls, style)
	}
	cls, style = g.resolveRateOverlayTone("#1a4a7a", darkPath, true)
	if !strings.Contains(cls, "rate-banner--custom") || !strings.Contains(style, "--rate-text:#1a4a7a") {
		t.Fatalf("custom hex: cls=%q style=%q", cls, style)
	}
}

func TestRenderRateBannerManualAndAuto(t *testing.T) {
	dir := t.TempDir()
	lightPath := filepath.Join(dir, "beach.png")
	writeSolidPNG(t, lightPath, color.NRGBA{R: 230, G: 220, B: 200, A: 255}, 48, 64)

	g := &Generator{}
	html := g.renderRateBanner(map[string]any{
		"form_key":   "fashion",
		"caption":    "FASHION",
		"price":      "20",
		"currency":   "$",
		"text_color": "",
	}, "/media/beach.png", lightPath, "eager", 48, 64)
	if !strings.Contains(html, "rate-banner--light") {
		t.Fatalf("auto should mark light banner with charcoal --light, got %s", html)
	}
	if strings.Contains(html, "rate-banner--custom") {
		t.Fatal("auto must not set custom")
	}
	if strings.Contains(html, "rate-banner--outline") {
		t.Fatal("auto light must use charcoal --light, not outline")
	}
	if !strings.Contains(html, `loading="eager"`) || !strings.Contains(html, `width="48"`) {
		t.Fatalf("eager first-row attrs missing: %s", html)
	}

	html = g.renderRateBanner(map[string]any{
		"form_key":   "fashion",
		"caption":    "FASHION",
		"text_color": "#ffffff",
	}, "/media/beach.png", lightPath, "lazy", 0, 0)
	if !strings.Contains(html, "rate-banner--custom") || !strings.Contains(html, `style="--rate-text:#ffffff;color:#ffffff`) {
		t.Fatalf("manual white override missing: %s", html)
	}
	if !strings.Contains(html, "rate-banner--text-backdrop") || !strings.Contains(html, "rate-banner__plate") {
		t.Fatalf("text backdrop should default on: %s", html)
	}

	html = g.renderRateBanner(map[string]any{
		"form_key":       "fashion",
		"caption":        "FASHION",
		"text_color":     "#1a4a7a",
		"text_backdrop":  false,
	}, "/media/beach.png", lightPath, "lazy", 0, 0)
	if !strings.Contains(html, "--rate-text:#1a4a7a") {
		t.Fatalf("custom color missing: %s", html)
	}
	if strings.Contains(html, "rate-banner--text-backdrop") {
		t.Fatal("text_backdrop false must omit plate class")
	}

	html = g.renderRateBanner(map[string]any{
		"form_key": "beauty",
		"caption":  "BEAUTY",
	}, "", "", "", 0, 0)
	if !strings.Contains(html, "is-placeholder") {
		t.Fatal("no image must be placeholder")
	}
	if !strings.Contains(html, "rate-banner--text-backdrop") {
		t.Fatal("missing text_backdrop still defaults on")
	}
	if strings.Contains(html, "rate-banner--outline") || strings.Contains(html, "rate-banner--light") || strings.Contains(html, "rate-banner--dark") {
		t.Fatalf("placeholder must not get auto tone class: %s", html)
	}
}
