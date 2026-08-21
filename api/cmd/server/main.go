package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"sheyanova.art/api/internal/auth"
	"sheyanova.art/api/internal/cms"
	"sheyanova.art/api/internal/generate"
	"sheyanova.art/api/internal/httpx"
	"sheyanova.art/api/internal/importfront"
)

func main() {
	cfg := loadConfig()
	if strings.TrimSpace(cfg.AdminToken) == "" {
		log.Fatal("ADMIN_TOKEN is required (set a non-empty value; local placeholders like change-me are OK)")
	}
	if err := os.MkdirAll(cfg.UploadDir, 0o755); err != nil {
		log.Fatalf("upload dir: %v", err)
	}
	if err := os.MkdirAll(cfg.PreviewDir, 0o755); err != nil {
		log.Fatalf("preview dir: %v", err)
	}
	if cfg.FrontDir != "" {
		if err := os.MkdirAll(cfg.FrontDir, 0o755); err != nil {
			log.Fatalf("front dir: %v", err)
		}
	}

	store, err := cms.Open(cfg.DataDir, cfg.UploadDir)
	if err != nil {
		log.Fatalf("cms db: %v", err)
	}
	defer store.Close()

	if err := store.BootSeed(); err != nil {
		log.Fatalf("seed: %v", err)
	}

	themeSrc := cfg.FrontThemeSrc
	if themeSrc == "" {
		// Prefer FRONT_DIR as theme kit when unset (compose mounts ./front → /front).
		if cfg.FrontDir != "" {
			if st, err := os.Stat(cfg.FrontDir); err == nil && st.IsDir() {
				themeSrc = cfg.FrontDir
			}
		}
		if themeSrc == "" {
			if abs, err := filepath.Abs("../front"); err == nil {
				if st, err := os.Stat(abs); err == nil && st.IsDir() {
					themeSrc = abs
				}
			}
		}
	}

	frontImportSrc := themeSrc
	if frontImportSrc == "" {
		frontImportSrc = cfg.FrontDir
	}
	runImport := func(force bool) (any, error) {
		return importfront.Import(store, frontImportSrc, force)
	}
	if shouldImportFront(cfg.ImportFront, frontImportSrc) {
		res, err := importfront.Import(store, frontImportSrc, false)
		if err != nil {
			log.Printf("cms: import-front skipped: %v", err)
		} else {
			log.Printf("cms: import-front pages_updated=%d media_created=%d skipped=%d",
				res.PagesUpdated, res.MediaCreated, res.PagesSkipped)
		}
	}

	gen, err := generate.New(store, generate.Config{
		OutDir:        cfg.PreviewDir,
		UploadDir:     cfg.UploadDir,
		ThemeSrc:      themeSrc,
		PreviewBase:   cfg.PreviewBaseURL,
		CanonicalBase: cfg.CanonicalBase,
	})
	if err != nil {
		log.Fatalf("generator: %v", err)
	}
	genSvc := &generate.Service{Gen: gen, FrontDir: cfg.FrontDir}
	cmsH := &cms.Handler{
		Store:     store,
		FrontDir:  frontImportSrc,
		MaxUpload: cfg.MaxUploadMB << 20,
		ImportFn:  runImport,
	}
	guard := auth.New(cfg.AdminToken)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
	})

	mux.Handle("/api/admin/me", guard.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "role": "admin"})
	})))

	// Admin CMS API (Bearer ADMIN_TOKEN)
	mux.Handle("/api/admin/settings", guard.Middleware(http.HandlerFunc(cmsH.Settings)))
	mux.Handle("/api/admin/nav", guard.Middleware(http.HandlerFunc(cmsH.Nav)))
	mux.Handle("/api/admin/pages", guard.Middleware(http.HandlerFunc(cmsH.Pages)))
	mux.Handle("/api/admin/pages/", guard.Middleware(http.HandlerFunc(cmsH.PageByID)))
	mux.Handle("/api/admin/media", guard.Middleware(http.HandlerFunc(cmsH.Media)))
	mux.Handle("/api/admin/media/", guard.Middleware(http.HandlerFunc(cmsH.MediaByID)))
	mux.Handle("/api/admin/generate", guard.Middleware(http.HandlerFunc(genSvc.HandleGenerate)))
	mux.Handle("/api/admin/preview/", guard.Middleware(http.HandlerFunc(genSvc.HandlePreviewPage)))
	mux.Handle("/api/admin/publish", guard.Middleware(http.HandlerFunc(genSvc.HandlePublish)))
	mux.Handle("/api/admin/publish/history", guard.Middleware(http.HandlerFunc(cmsH.PublishHistory)))
	mux.Handle("/api/admin/seed", guard.Middleware(http.HandlerFunc(cmsH.Seed)))
	mux.Handle("/api/admin/import-front", guard.Middleware(http.HandlerFunc(cmsH.ImportFront)))

	mux.Handle("/media/", http.StripPrefix("/media/", http.FileServer(http.Dir(cfg.UploadDir))))
	mux.Handle("/preview/", http.StripPrefix("/preview/", http.FileServer(http.Dir(cfg.PreviewDir))))

	handler := withCORS(cfg.CORSOrigins, mux)

	log.Printf("sheyanova cms api listening on %s", cfg.ListenAddr)
	log.Printf("data=%s uploads=%s preview=%s front=%s theme=%s", cfg.DataDir, cfg.UploadDir, cfg.PreviewDir, cfg.FrontDir, themeSrc)
	if err := http.ListenAndServe(cfg.ListenAddr, handler); err != nil {
		log.Fatal(err)
	}
}

type config struct {
	ListenAddr     string
	DataDir        string
	UploadDir      string
	PreviewDir     string
	PreviewBaseURL string
	FrontDir       string
	FrontThemeSrc  string
	ImportFront    string
	CanonicalBase  string
	CORSOrigins    []string
	AdminToken     string
	MaxUploadMB    int64
}

func loadConfig() config {
	maxMB, _ := strconv.ParseInt(envOr("MAX_UPLOAD_MB", "25"), 10, 64)
	if maxMB <= 0 {
		maxMB = 25
	}
	origins := strings.Split(envOr("CORS_ORIGINS", "*"), ",")
	for i := range origins {
		origins[i] = strings.TrimSpace(origins[i])
	}
	data := envOr("DATA_DIR", "./data")
	upload := envOr("UPLOAD_DIR", filepath.Join(data, "uploads"))
	preview := envOr("PREVIEW_DIR", filepath.Join(data, "preview"))
	front := envOr("FRONT_DIR", "")
	theme := envOr("FRONT_THEME_SRC", "")
	if theme == "" && front != "" {
		theme = front
	}
	return config{
		ListenAddr:     envOr("LISTEN_ADDR", ":8080"),
		DataDir:        data,
		UploadDir:      upload,
		PreviewDir:     preview,
		PreviewBaseURL: envOr("PREVIEW_BASE_URL", "/preview"),
		FrontDir:       front,
		FrontThemeSrc:  theme,
		ImportFront:    envOr("IMPORT_FRONT", ""),
		CanonicalBase:  envOr("CANONICAL_BASE", "https://sheyanova.art"),
		CORSOrigins:    origins,
		AdminToken:     loadAdminToken(),
		MaxUploadMB:    maxMB,
	}
}

func shouldImportFront(flag, frontDir string) bool {
	switch strings.ToLower(strings.TrimSpace(flag)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	}
	// Auto: when unset, import once if front looks like the static site.
	return importfront.HasFrontContent(frontDir)
}

func loadAdminToken() string {
	v, set := os.LookupEnv("ADMIN_TOKEN")
	if set && strings.TrimSpace(v) == "" {
		// Explicit empty is a misconfig; unset still defaults for local.
		return ""
	}
	if strings.TrimSpace(v) != "" {
		return v
	}
	return "change-me"
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func withCORS(origins []string, next http.Handler) http.Handler {
	allowAll := len(origins) == 1 && origins[0] == "*"
	allowed := map[string]struct{}{}
	for _, o := range origins {
		if o != "" && o != "*" {
			allowed[o] = struct{}{}
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if allowAll {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		} else if _, ok := allowed[origin]; ok {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
