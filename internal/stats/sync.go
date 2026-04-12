package stats

import (
	"context"
	"os"
	"path/filepath"
	"time"
)

// Sync queries the running Xray instance for live traffic counters, updates
// the persistent store, and saves it to disk. It returns the updated store.
func Sync(ctx context.Context, xrayBinary, apiAddress, storePath string) (*Store, error) {
	traffic, err := QueryUserTraffic(ctx, xrayBinary, apiAddress)
	if err != nil {
		return nil, err
	}

	store, err := LoadStore(storePath)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	for _, ut := range traffic {
		store.Update(ut.Name, ut.Upload, ut.Download, now)
	}

	if err := os.MkdirAll(filepath.Dir(storePath), 0o755); err != nil {
		return nil, err
	}
	if err := store.Save(storePath); err != nil {
		return nil, err
	}
	return store, nil
}
