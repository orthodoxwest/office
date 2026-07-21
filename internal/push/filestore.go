package push

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// FileStore is a JSON-file-backed Store guarded by a mutex. It keeps the whole
// subscription set in memory and rewrites the file atomically on every change.
// This suits a single server instance with a modest number of subscribers; if
// the app ever scales to multiple instances, swap in a shared database behind
// the same Store interface.
type FileStore struct {
	path string
	mu   sync.Mutex
	recs map[string]Record // keyed by endpoint
}

// NewFileStore opens (or initializes) the store at path, loading any existing
// subscriptions. A missing file is not an error — it starts empty.
func NewFileStore(path string) (*FileStore, error) {
	fs := &FileStore{path: path, recs: map[string]Record{}}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fs, nil
		}
		return nil, fmt.Errorf("push: reading store %s: %w", path, err)
	}
	if len(data) == 0 {
		return fs, nil
	}
	var recs []Record
	if err := json.Unmarshal(data, &recs); err != nil {
		return nil, fmt.Errorf("push: parsing store %s: %w", path, err)
	}
	for _, r := range recs {
		fs.recs[r.Subscription.Endpoint] = r
	}
	return fs, nil
}

// flush writes the current records to disk atomically. The caller holds mu.
func (fs *FileStore) flush() error {
	recs := make([]Record, 0, len(fs.recs))
	for _, r := range fs.recs {
		recs = append(recs, r)
	}
	data, err := json.MarshalIndent(recs, "", "  ")
	if err != nil {
		return fmt.Errorf("push: encoding store: %w", err)
	}
	dir := filepath.Dir(fs.path)
	tmp, err := os.CreateTemp(dir, ".push-*.tmp")
	if err != nil {
		return fmt.Errorf("push: creating temp store: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("push: writing temp store: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("push: closing temp store: %w", err)
	}
	if err := os.Rename(tmpName, fs.path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("push: replacing store: %w", err)
	}
	return nil
}

// Put inserts or replaces rec.
func (fs *FileStore) Put(rec Record) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.recs[rec.Subscription.Endpoint] = rec
	return fs.flush()
}

// Delete removes the record for endpoint; a missing endpoint is a no-op.
func (fs *FileStore) Delete(endpoint string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	if _, ok := fs.recs[endpoint]; !ok {
		return nil
	}
	delete(fs.recs, endpoint)
	return fs.flush()
}

// Get returns the record for endpoint.
func (fs *FileStore) Get(endpoint string) (Record, bool, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	rec, ok := fs.recs[endpoint]
	return rec, ok, nil
}

// All returns a snapshot of every stored record.
func (fs *FileStore) All() ([]Record, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	recs := make([]Record, 0, len(fs.recs))
	for _, r := range fs.recs {
		recs = append(recs, r)
	}
	return recs, nil
}
