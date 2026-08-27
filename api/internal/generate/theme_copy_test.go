package generate

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCopyThemeAssetsSkipsCDN(t *testing.T) {
	theme := t.TempDir()
	out := t.TempDir()

	mustWrite := func(rel, body string) {
		t.Helper()
		p := filepath.Join(theme, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite("assets/theme/app.css", "theme-kit")
	mustWrite("assets/cdn/huge.jpg", "do-not-copy-me")
	mustWrite("static/x.js", "static-kit")
	mustWrite("fonts/a.woff2", "font-kit")

	g, err := New(nil, Config{OutDir: out, ThemeSrc: theme})
	if err != nil {
		t.Fatal(err)
	}
	if err := g.copyThemeAssets(); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(out, "assets", "theme", "app.css")); err != nil {
		t.Fatalf("expected theme kit copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, "static", "x.js")); err != nil {
		t.Fatalf("expected static copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, "fonts", "a.woff2")); err != nil {
		t.Fatalf("expected fonts copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, "assets", "cdn", "huge.jpg")); !os.IsNotExist(err) {
		t.Fatalf("assets/cdn must not be bulk-copied, got err=%v", err)
	}
}

func TestCopyThemeAssetsSkipUnchanged(t *testing.T) {
	theme := t.TempDir()
	out := t.TempDir()
	srcFile := filepath.Join(theme, "assets", "theme", "app.css")
	if err := os.MkdirAll(filepath.Dir(srcFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(srcFile, []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	mtime := time.Now().Add(-time.Hour).Truncate(time.Second)
	_ = os.Chtimes(srcFile, mtime, mtime)

	g, err := New(nil, Config{OutDir: out, ThemeSrc: theme})
	if err != nil {
		t.Fatal(err)
	}
	if err := g.copyThemeAssets(); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(out, "assets", "theme", "app.css")
	st1, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}

	// Touch destination content marker, then re-copy — skip should leave bytes intact
	// only when size+mtime match; rewrite dst mtime to match src so skip triggers.
	if err := os.WriteFile(dst, []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = os.Chtimes(dst, mtime, mtime)
	before, _ := os.Stat(dst)

	if err := g.copyThemeAssets(); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if !after.ModTime().Equal(before.ModTime()) || after.Size() != st1.Size() {
		t.Fatalf("expected skip-unchanged to preserve dst mtime/size")
	}

	// Source change must update destination.
	if err := os.WriteFile(srcFile, []byte("v2-longer"), 0o644); err != nil {
		t.Fatal(err)
	}
	newMT := time.Now().Truncate(time.Second)
	_ = os.Chtimes(srcFile, newMT, newMT)
	if err := g.copyThemeAssets(); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "v2-longer" {
		t.Fatalf("expected updated kit file, got %q", body)
	}
}

func TestCopyThemeAssetsSelfCopySkip(t *testing.T) {
	dir := t.TempDir()
	cdn := filepath.Join(dir, "assets", "cdn", "x.jpg")
	if err := os.MkdirAll(filepath.Dir(cdn), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cdn, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	g, err := New(nil, Config{OutDir: dir, ThemeSrc: dir})
	if err != nil {
		t.Fatal(err)
	}
	if err := g.copyThemeAssets(); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(cdn)
	if err != nil || string(body) != "keep" {
		t.Fatalf("publish self-copy must not truncate theme, got %q err=%v", body, err)
	}
}

func TestWriteArticleCSSOnlyOutDir(t *testing.T) {
	theme := t.TempDir()
	out := t.TempDir()
	themeCSS := filepath.Join(theme, "assets", "theme", "about.css")
	if err := os.MkdirAll(filepath.Dir(themeCSS), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(themeCSS, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	g, err := New(nil, Config{OutDir: out, ThemeSrc: theme})
	if err != nil {
		t.Fatal(err)
	}
	if err := g.writeArticleCSS(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(out, "assets", "theme", "about.css")); err != nil {
		t.Fatalf("expected about.css in OutDir: %v", err)
	}
	body, err := os.ReadFile(themeCSS)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "original" {
		t.Fatalf("ThemeSrc about.css must stay untouched on draft path, got %q", body)
	}
}
