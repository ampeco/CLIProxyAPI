package registry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// The only test in the package that starts the real updater: its start and
// finish signals are process-wide and close once.
func TestWaitForStartupModelRefresh(t *testing.T) {
	t.Run("returns at once when the updater was never started", func(t *testing.T) {
		start := time.Now()
		if WaitForStartupModelRefresh(context.Background(), 5*time.Second) {
			t.Fatal("reported a finished fetch although the updater never started")
		}
		if elapsed := time.Since(start); elapsed > time.Second {
			t.Fatalf("waited %s for an updater that was never started", elapsed)
		}
	})

	t.Run("returns once the first fetch has published the remote catalog", func(t *testing.T) {
		var catalog map[string][]map[string]any
		if err := json.Unmarshal(embeddedModelsJSON, &catalog); err != nil {
			t.Fatalf("decode embedded catalog: %v", err)
		}
		catalog["claude"] = append(catalog["claude"], map[string]any{"id": "claude-startup-wait-test", "object": "model"})
		body, err := json.Marshal(catalog)
		if err != nil {
			t.Fatalf("encode catalog: %v", err)
		}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			time.Sleep(200 * time.Millisecond)
			_, _ = w.Write(body)
		}))
		defer server.Close()

		previousURLs, previousData := modelsURLs, getModels()
		modelsURLs = []string{server.URL}
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(func() {
			cancel()
			modelsURLs = previousURLs
			modelsCatalogStore.mu.Lock()
			modelsCatalogStore.data = previousData
			modelsCatalogStore.mu.Unlock()
		})

		StartModelsUpdater(ctx)
		if !WaitForStartupModelRefresh(context.Background(), 5*time.Second) {
			t.Fatal("first fetch did not finish within 5s")
		}
		found := false
		for _, model := range getModels().Claude {
			found = found || model.ID == "claude-startup-wait-test"
		}
		if !found {
			t.Fatal("wait returned before the fetched catalog was published")
		}
	})
}
