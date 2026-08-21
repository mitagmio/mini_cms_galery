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
	"time"

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
	pushStatus, pushDetail := PushFrontRepo(publishDir, body.Note)
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

// PushFrontRepo commits the already-generated FRONT_DIR tree in place and pushes
// to GitHub via HTTPS + GH_TOKEN (PAT), so the server checkout stays in sync with origin.
func PushFrontRepo(frontDir, note string) (status string, detail map[string]any) {
	repo := firstEnv("GITHUB_REPO", "GHP_REPO")
	branch := firstEnv("GITHUB_BRANCH", "GHP_BRANCH")
	if branch == "" {
		branch = "main"
	}
	token := firstEnv("GITHUB_TOKEN", "GH_TOKEN")

	detail = map[string]any{
		"repo":   repo,
		"branch": branch,
		"source": frontDir,
		"mode":   "in-place",
		"auth":   "https-token",
	}

	if frontDir == "" {
		detail["error"] = "FRONT_DIR empty"
		return "error", detail
	}
	if _, err := os.Stat(filepath.Join(frontDir, ".git")); err != nil {
		detail["error"] = "FRONT_DIR is not a git checkout (.git missing)"
		return "error", detail
	}
	if repo == "" {
		detail["todo"] = "Set GITHUB_REPO (owner/name)"
		detail["message"] = "git push stubbed — repo unset"
		return "stub", detail
	}
	if token == "" {
		detail["todo"] = "Set GH_TOKEN (or GITHUB_TOKEN)"
		detail["message"] = "git push stubbed — token unset"
		log.Printf("publish: git push stubbed (GH_TOKEN unset)")
		return "stub", detail
	}

	env := append([]string{}, os.Environ()...)
	env = append(env, "GIT_TERMINAL_PROMPT=0")

	// Embed PAT in origin URL for fetch/push (local repo only; not committed).
	remote := fmt.Sprintf("https://x-access-token:%s@github.com/%s.git", token, repo)

	_ = runCmdEnv(env, "git", "-C", frontDir, "config", "user.email", "cms@sheyanova.art")
	_ = runCmdEnv(env, "git", "-C", frontDir, "config", "user.name", "Sheyanova CMS")
	_ = runCmdEnv(env, "git", "-C", frontDir, "remote", "set-url", "origin", remote)

	_ = runCmdEnv(env, "git", "-C", frontDir, "add", "-A")
	msg := "cms publish"
	if strings.TrimSpace(note) != "" {
		msg = "cms publish: " + strings.TrimSpace(note)
	}
	msg = fmt.Sprintf("%s\n\nGenerated at %s", msg, time.Now().UTC().Format(time.RFC3339))

	commitErr := runCmdEnv(env, "git", "-C", frontDir, "commit", "-m", msg)
	if commitErr != nil {
		detail["message"] = "nothing to commit or commit failed"
		detail["commit_error"] = commitErr.Error()
	}

	_ = runCmdEnv(env, "git", "-C", frontDir, "fetch", "origin", branch)
	if err := runCmdEnv(env, "git", "-C", frontDir, "pull", "--rebase", "origin", branch); err != nil {
		detail["error"] = "git pull --rebase failed: " + err.Error()
		_ = runCmdEnv(env, "git", "-C", frontDir, "rebase", "--abort")
		return "error", detail
	}

	sha, _ := cmdOutput(env, "git", "-C", frontDir, "rev-parse", "HEAD")
	detail["commit"] = strings.TrimSpace(sha)

	if err := runCmdEnv(env, "git", "-C", frontDir, "push", "origin", branch); err != nil {
		detail["error"] = err.Error()
		// Best-effort: strip token from origin even on failure.
		_ = runCmdEnv(env, "git", "-C", frontDir, "remote", "set-url", "origin", fmt.Sprintf("git@github.com:%s.git", repo))
		return "error", detail
	}

	// Restore SSH remote for host/interactive use (don't leave PAT in .git/config).
	_ = runCmdEnv(env, "git", "-C", frontDir, "remote", "set-url", "origin", fmt.Sprintf("git@github.com:%s.git", repo))

	detail["message"] = "pushed"
	return "ok", detail
}

func firstEnv(keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	return ""
}

func runCmd(name string, args ...string) error {
	return runCmdEnv(nil, name, args...)
}

func runCmdEnv(env []string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	if env != nil {
		cmd.Env = env
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func cmdOutput(env []string, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	if env != nil {
		cmd.Env = env
	}
	out, err := cmd.Output()
	return string(out), err
}

// syncDir kept for any legacy callers / tests.
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
		if rel == ".git" || strings.HasPrefix(rel, ".git"+string(os.PathSeparator)) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target)
	})
}
