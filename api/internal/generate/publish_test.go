package generate

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"sheyanova.art/api/internal/cms"
)

func TestAsyncPublishReturnsJobAndPolls(t *testing.T) {
	store := testStore(t)
	if _, err := store.PutSettings(cms.SiteSettings{SiteName: "Test", CanonicalBase: "https://www.sheyanova.art"}); err != nil {
		t.Fatal(err)
	}
	if err := store.EnsureSystemTemplates(); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreatePage(cms.Page{Slug: "about", Title: "ABOUT", Theme: cms.ThemeTextContent, Status: "published"}); err != nil {
		t.Fatal(err)
	}

	frontDir := t.TempDir()
	g, err := New(store, Config{
		OutDir:       t.TempDir(),
		UploadDir:    filepath.Join(t.TempDir(), "up"),
		PreviewBase:  "/preview",
		PathPrefix:   "/preview",
		PublicAPIURL: "https://api.sheyanova.art",
	})
	if err != nil {
		t.Fatal(err)
	}

	var pushCalls atomic.Int32
	var muHeldDuringPush atomic.Bool
	svc := &Service{Gen: g, FrontDir: frontDir}
	svc.pushFront = func(frontDir, note string) (string, map[string]any) {
		pushCalls.Add(1)
		acquired := make(chan struct{})
		go func() {
			svc.mu.Lock()
			close(acquired)
			svc.mu.Unlock()
		}()
		select {
		case <-acquired:
			// generate lock released before push — expected
		case <-time.After(2 * time.Second):
			muHeldDuringPush.Store(true)
		}
		time.Sleep(50 * time.Millisecond)
		return "ok", map[string]any{"message": "pushed", "commit": "deadbeef"}
	}

	req := httptest.NewRequest(http.MethodPost, "/api/admin/publish", nil)
	rec := httptest.NewRecorder()
	started := time.Now()
	svc.HandlePublish(rec, req)
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("POST publish blocked too long: %v", elapsed)
	}
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var postBody map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &postBody); err != nil {
		t.Fatal(err)
	}
	jobID, _ := postBody["job_id"].(string)
	status, _ := postBody["status"].(string)
	if jobID == "" || (status != "queued" && status != "running") {
		t.Fatalf("unexpected post body: %#v", postBody)
	}

	// Concurrent publish should reuse the active job.
	req2 := httptest.NewRequest(http.MethodPost, "/api/admin/publish", nil)
	rec2 := httptest.NewRecorder()
	svc.HandlePublish(rec2, req2)
	if rec2.Code != http.StatusAccepted {
		t.Fatalf("concurrent status=%d", rec2.Code)
	}
	var post2 map[string]any
	_ = json.Unmarshal(rec2.Body.Bytes(), &post2)
	if post2["job_id"] != jobID {
		t.Fatalf("expected same job_id, got %v vs %s", post2["job_id"], jobID)
	}

	deadline := time.Now().Add(10 * time.Second)
	var finalStatus string
	for time.Now().Before(deadline) {
		greq := httptest.NewRequest(http.MethodGet, "/api/admin/publish/jobs/"+jobID, nil)
		grec := httptest.NewRecorder()
		svc.HandlePublishJob(grec, greq)
		if grec.Code != http.StatusOK {
			t.Fatalf("poll status=%d body=%s", grec.Code, grec.Body.String())
		}
		var poll map[string]any
		if err := json.Unmarshal(grec.Body.Bytes(), &poll); err != nil {
			t.Fatal(err)
		}
		finalStatus, _ = poll["status"].(string)
		if !isActivePublishStatus(finalStatus) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if finalStatus != "ok" {
		t.Fatalf("final status=%s", finalStatus)
	}
	if pushCalls.Load() != 1 {
		t.Fatalf("push calls=%d", pushCalls.Load())
	}
	if muHeldDuringPush.Load() {
		t.Fatal("generate mutex still held during git push")
	}

	// After completion a new publish should start a new job.
	var wg sync.WaitGroup
	wg.Add(1)
	svc.pushFront = func(frontDir, note string) (string, map[string]any) {
		defer wg.Done()
		return "stub", map[string]any{"message": "stubbed"}
	}
	req3 := httptest.NewRequest(http.MethodPost, "/api/admin/publish", nil)
	rec3 := httptest.NewRecorder()
	svc.HandlePublish(rec3, req3)
	var post3 map[string]any
	_ = json.Unmarshal(rec3.Body.Bytes(), &post3)
	if post3["job_id"] == jobID {
		t.Fatal("expected a new job after completion")
	}
	wg.Wait()
}
