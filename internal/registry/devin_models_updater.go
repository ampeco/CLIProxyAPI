package registry

import (
	"context"
	"io"
	"net/http"
	"sync"

	log "github.com/sirupsen/logrus"
)

const maxDevinModelsSize = 8 << 20

var devinModelsURLs = []string{
	"https://raw.githubusercontent.com/router-for-me/models/refs/heads/main/devin_models.json",
	"https://models.router-for.me/devin_models.json",
}

var devinModelsUpdaterOnce sync.Once

// StartDevinModelsUpdater starts a background updater that fetches the
// Devin model catalog immediately and refreshes it every 3 hours.
// Safe to call multiple times; only one updater runs.
func StartDevinModelsUpdater(ctx context.Context) {
	devinModelsUpdaterOnce.Do(func() {
		go runDevinModelsUpdater(ctx)
	})
}

func runDevinModelsUpdater(ctx context.Context) {
	runModelCatalogUpdater(ctx, "Devin model refresh", tryRefreshDevinModels)
}

func tryRefreshDevinModels(ctx context.Context, label string) bool {
	data, sourceURL := fetchDevinModelsFromRemote(ctx)
	if data == nil {
		log.Warnf("%s: fetch failed from all URLs, keeping current data (embedded or cached fallback)", label)
		return false
	}

	changed, err := loadDevinModelsFromBytes(data, sourceURL)
	if err != nil {
		log.Warnf("%s: fetched catalog rejected, keeping current data: %v", label, err)
		return false
	}
	if !changed {
		log.Infof("%s completed from %s, no changes detected", label, sourceURL)
		return true
	}
	log.Infof("%s completed from %s, catalog updated", label, sourceURL)
	return true
}

func fetchDevinModelsFromRemote(ctx context.Context) ([]byte, string) {
	client := &http.Client{Timeout: modelsFetchTimeout}
	for _, sourceURL := range devinModelsURLs {
		reqCtx, cancel := context.WithTimeout(ctx, modelsFetchTimeout)
		req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, sourceURL, nil)
		if err != nil {
			cancel()
			log.Warnf("devin models updater: invalid request for %s: %v", sourceURL, err)
			continue
		}

		resp, err := client.Do(req)
		cancel()
		if err != nil {
			log.Warnf("devin models updater: fetch failed from %s: %v", sourceURL, err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			log.Warnf("devin models updater: unexpected status %d from %s", resp.StatusCode, sourceURL)
			_ = resp.Body.Close()
			continue
		}

		body, err := io.ReadAll(io.LimitReader(resp.Body, maxDevinModelsSize))
		_ = resp.Body.Close()
		if err != nil {
			log.Warnf("devin models updater: read failed from %s: %v", sourceURL, err)
			continue
		}

		return body, sourceURL
	}
	return nil, ""
}
