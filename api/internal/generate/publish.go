package generate

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"sheyanova.art/api/internal/cms"
	"sheyanova.art/api/internal/httpx"
)

type Service struct {
	Gen      *Generator
	FrontDir string // publish output (FRONT_DIR); draft generate stays on Gen.Cfg.OutDir
}

func (s *Service) HandleGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpx.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	prevPrefix := s.Gen.Cfg.PathPrefix
	s.Gen.Cfg.PathPrefix = strings.TrimRight(s.Gen.Cfg.PreviewBase, "/")
	err := s.Gen.GenerateSite()
	s.Gen.Cfg.PathPrefix = prevPrefix
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	url := strings.TrimRight(s.Gen.Cfg.PreviewBase, "/") + "/"
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"ok":          true,
		"outDir":      s.Gen.Cfg.OutDir,
		"url":         url,
		"preview_url": url,
		"message":     "Draft site generated",
	})
}

func (s *Service) HandlePreviewPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpx.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/admin/preview/")
	id = strings.Trim(id, "/")
	if id == "" {
		httpx.WriteError(w, http.StatusBadRequest, "page id required")
		return
	}
	prevPrefix := s.Gen.Cfg.PathPrefix
	s.Gen.Cfg.PathPrefix = strings.TrimRight(s.Gen.Cfg.PreviewBase, "/")
	url, err := s.Gen.GeneratePage(id)
	s.Gen.Cfg.PathPrefix = prevPrefix
	if err != nil {
		if err == cms.ErrNotFound {
			httpx.WriteError(w, http.StatusNotFound, "not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"ok": true, "url": url, "preview_url": url,
	})
}

func (s *Service) HandlePublish(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpx.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		Note string `json:"note"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	prevOut := s.Gen.Cfg.OutDir
	prevPrefix := s.Gen.Cfg.PathPrefix
	if s.FrontDir != "" {
		s.Gen.Cfg.OutDir = s.FrontDir
	}
	s.Gen.Cfg.PathPrefix = "" // public GHP URLs are site-root absolute
	err := s.Gen.GenerateSite()
	s.Gen.Cfg.OutDir = prevOut
	s.Gen.Cfg.PathPrefix = prevPrefix
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	publishDir := s.FrontDir
	if publishDir == "" {
		publishDir = prevOut
	}
	pushStatus, pushDetail := PushToGitHub(publishDir)
	detail := cms.MustJSON(map[string]any{
		"outDir": publishDir,
		"git":    pushDetail,
	})
	h, err := s.Gen.Store.AddPublishHistory(cms.PublishHistory{
		Note:   body.Note,
		Status: pushStatus,
		Detail: detail,
	})
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"history": h,
		"git":     pushDetail,
	})
}

// PushToGitHub is a skeleton for publishing generated files to GitHub Pages / repo.
// When GITHUB_TOKEN / GITHUB_REPO are unset, returns stub status with TODO.
func PushToGitHub(generatedDir string) (status string, detail map[string]any) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		token = os.Getenv("GH_TOKEN")
	}
	repo := os.Getenv("GITHUB_REPO") // e.g. owner/repo
	if repo == "" {
		repo = os.Getenv("GHP_REPO")
	}
	branch := os.Getenv("GITHUB_BRANCH")
	if branch == "" {
		branch = os.Getenv("GHP_BRANCH")
	}
	if branch == "" {
		branch = "main"
	}
	targetDir := os.Getenv("GITHUB_TARGET_DIR")
	if targetDir == "" {
		targetDir = "."
	}

	detail = map[string]any{
		"repo":       repo,
		"branch":     branch,
		"target_dir": targetDir,
		"source":     generatedDir,
	}

	if token == "" || repo == "" {
		detail["todo"] = "Set GH_TOKEN (or GITHUB_TOKEN) and GITHUB_REPO to enable git push publish"
		detail["message"] = "git push stubbed — credentials unset"
		log.Printf("publish: git push stubbed (GH_TOKEN/GITHUB_TOKEN or repo unset)")
		return "stub", detail
	}

	// Skeleton implementation: clone/update worktree and copy files, then push.
	work := filepath.Join(os.TempDir(), "sheyanova-publish")
	_ = os.RemoveAll(work)
	remote := fmt.Sprintf("https://x-access-token:%s@github.com/%s.git", token, repo)
	if err := runCmd("git", "clone", "--depth", "1", "--branch", branch, remote, work); err != nil {
		// try orphan branch create
		if err2 := runCmd("git", "clone", "--depth", "1", remote, work); err2 != nil {
			detail["error"] = err.Error()
			return "error", detail
		}
		_ = runCmd("git", "-C", work, "checkout", "--orphan", branch)
	}

	dest := filepath.Join(work, targetDir)
	if targetDir == "." {
		dest = work
	}
	if err := syncDir(generatedDir, dest); err != nil {
		detail["error"] = err.Error()
		return "error", detail
	}
	_ = runCmd("git", "-C", work, "config", "user.email", "cms@sheyanova.art")
	_ = runCmd("git", "-C", work, "config", "user.name", "Sheyanova CMS")
	_ = runCmd("git", "-C", work, "add", "-A")
	if err := runCmd("git", "-C", work, "commit", "-m", "cms publish"); err != nil {
		detail["message"] = "nothing to commit or commit failed"
		detail["commit_error"] = err.Error()
	}
	if err := runCmd("git", "-C", work, "push", "origin", branch); err != nil {
		detail["error"] = err.Error()
		return "error", detail
	}
	detail["message"] = "pushed"
	return "ok", detail
}

func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func syncDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target)
	})
}
