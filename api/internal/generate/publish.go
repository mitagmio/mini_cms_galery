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
	"sync"
	"time"

	"sheyanova.art/api/internal/cms"
	"sheyanova.art/api/internal/httpx"
)

type Service struct {
	Gen      *Generator
	FrontDir string // publish output (FRONT_DIR); draft generate stays on Gen.Cfg.OutDir
	mu       sync.Mutex

	// publishMu serializes publish jobs (one at a time). Generate lock (mu) is only
	// held around filesystem GenerateSite — not across git push.
	publishMu   sync.Mutex
	activeJobID string
	// pushFront defaults to PushFrontRepo; tests may stub to avoid real git.
	pushFront func(frontDir, note string) (status string, detail map[string]any)
}

// GeneratePreview writes the draft site into OutDir with PathPrefix=/preview.
func (s *Service) GeneratePreview() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	prevPrefix := s.Gen.Cfg.PathPrefix
	s.Gen.Cfg.PathPrefix = strings.TrimRight(s.Gen.Cfg.PreviewBase, "/")
	s.Gen.Cfg.PublishedOnly = false
	err := s.Gen.GenerateSite()
	s.Gen.Cfg.PathPrefix = prevPrefix
	return err
}

func (s *Service) HandleGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpx.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		PageID string `json:"page_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	pageID := strings.TrimSpace(body.PageID)

	// Single-page draft when page_id is set (fast path for editor Generate draft).
	if pageID != "" {
		s.mu.Lock()
		prevPrefix := s.Gen.Cfg.PathPrefix
		s.Gen.Cfg.PathPrefix = strings.TrimRight(s.Gen.Cfg.PreviewBase, "/")
		url, err := s.Gen.GeneratePage(pageID)
		s.Gen.Cfg.PathPrefix = prevPrefix
		s.mu.Unlock()
		if err != nil {
			if err == cms.ErrNotFound {
				httpx.WriteError(w, http.StatusNotFound, "not found")
				return
			}
			httpx.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{
			"ok":          true,
			"outDir":      s.Gen.Cfg.OutDir,
			"url":         url,
			"preview_url": url,
			"message":     "Draft page generated",
		})
		return
	}

	if err := s.GeneratePreview(); err != nil {
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
	s.mu.Lock()
	prevPrefix := s.Gen.Cfg.PathPrefix
	s.Gen.Cfg.PathPrefix = strings.TrimRight(s.Gen.Cfg.PreviewBase, "/")
	url, err := s.Gen.GeneratePage(id)
	s.Gen.Cfg.PathPrefix = prevPrefix
	s.mu.Unlock()
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
		Note   string `json:"note"`
		PageID string `json:"page_id"` // ignored — publish always regenerates the full site
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	_ = body.PageID

	s.publishMu.Lock()
	if s.activeJobID != "" {
		jobID := s.activeJobID
		s.publishMu.Unlock()
		if h, err := s.Gen.Store.GetPublishHistory(jobID); err == nil && isActivePublishStatus(h.Status) {
			httpx.WriteJSON(w, http.StatusAccepted, map[string]any{
				"ok":      true,
				"job_id":  h.ID,
				"status":  h.Status,
				"history": h,
				"message": "publish already in progress",
			})
			return
		}
		s.publishMu.Lock()
		if s.activeJobID == jobID {
			s.activeJobID = ""
		}
	}

	h, err := s.Gen.Store.AddPublishHistory(cms.PublishHistory{
		Note:   body.Note,
		Status: "queued",
		Detail: cms.MustJSON(map[string]any{
			"stage":   "queued",
			"message": "full site publish queued",
		}),
	})
	if err != nil {
		s.publishMu.Unlock()
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.activeJobID = h.ID
	s.publishMu.Unlock()

	go s.runPublishJob(h.ID, body.Note)

	httpx.WriteJSON(w, http.StatusAccepted, map[string]any{
		"ok":      true,
		"job_id":  h.ID,
		"status":  "queued",
		"history": h,
		"message": "Publish queued (full site)",
	})
}

func (s *Service) HandlePublishJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httpx.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/admin/publish/jobs/")
	id = strings.Trim(id, "/")
	if id == "" {
		httpx.WriteError(w, http.StatusBadRequest, "job id required")
		return
	}
	h, err := s.Gen.Store.GetPublishHistory(id)
	if err != nil {
		if err == cms.ErrNotFound {
			httpx.WriteError(w, http.StatusNotFound, "not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	stage := publishStage(h.Detail)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"job_id":  h.ID,
		"status":  h.Status,
		"stage":   stage,
		"history": h,
		"job":     h,
	})
}

func isActivePublishStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "queued", "running":
		return true
	default:
		return false
	}
}

func publishStage(detail json.RawMessage) string {
	if len(detail) == 0 {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal(detail, &m); err != nil {
		return ""
	}
	if s, ok := m["stage"].(string); ok {
		return s
	}
	return ""
}

func (s *Service) updatePublishJob(id, status string, detail map[string]any) {
	if _, err := s.Gen.Store.UpdatePublishHistory(id, status, cms.MustJSON(detail)); err != nil {
		log.Printf("publish: update job %s status=%s: %v", id, status, err)
	}
}

func (s *Service) runPublishJob(jobID, note string) {
	defer func() {
		if rec := recover(); rec != nil {
			log.Printf("publish: job %s panic: %v", jobID, rec)
			s.updatePublishJob(jobID, "error", map[string]any{
				"stage":   "error",
				"message": "publish panicked",
			})
		}
		s.publishMu.Lock()
		if s.activeJobID == jobID {
			s.activeJobID = ""
		}
		s.publishMu.Unlock()
	}()

	s.updatePublishJob(jobID, "running", map[string]any{
		"stage":   "generate",
		"message": "generating published site",
	})

	s.mu.Lock()
	prevOut := s.Gen.Cfg.OutDir
	prevPrefix := s.Gen.Cfg.PathPrefix
	if s.FrontDir != "" {
		s.Gen.Cfg.OutDir = s.FrontDir
	}
	s.Gen.Cfg.PathPrefix = "" // public GHP URLs are site-root absolute
	s.Gen.Cfg.PublishedOnly = true
	err := s.Gen.GenerateSite()
	s.Gen.Cfg.OutDir = prevOut
	s.Gen.Cfg.PathPrefix = prevPrefix
	s.Gen.Cfg.PublishedOnly = false
	s.mu.Unlock()

	publishDir := s.FrontDir
	if publishDir == "" {
		publishDir = prevOut
	}
	if err != nil {
		s.updatePublishJob(jobID, "error", map[string]any{
			"stage":   "generate",
			"outDir":  publishDir,
			"error":   err.Error(),
			"message": "generate failed",
		})
		return
	}

	s.updatePublishJob(jobID, "running", map[string]any{
		"stage":   "push",
		"outDir":  publishDir,
		"message": "pushing to github",
	})

	pushFn := s.pushFront
	if pushFn == nil {
		pushFn = PushFrontRepo
	}
	pushStatus, pushDetail := pushFn(publishDir, note)
	detail := map[string]any{
		"stage":  "done",
		"outDir": publishDir,
		"git":    pushDetail,
	}
	if msg, ok := pushDetail["message"].(string); ok && msg != "" {
		detail["message"] = msg
	}
	if errMsg, ok := pushDetail["error"].(string); ok && errMsg != "" {
		detail["error"] = errMsg
	}
	s.updatePublishJob(jobID, pushStatus, detail)
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
