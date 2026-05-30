package cache

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

// ErrLocked is returned when a model cache entry is already locked.
var ErrLocked = errors.New("model cache entry is locked")

// lockFile is the on-disk representation of a cache lock.
type lockFile struct {
	Owner    string    `json:"owner"`
	Acquired time.Time `json:"acquired"`
}

// AcquireModelLock writes a lock.json for a model cache entry. It is a
// best-effort coarse lock guarding concurrent pulls of the same model within a
// single runtime process; it is not a distributed lock.
func (l *Layout) AcquireModelLock(namespace, name, owner string) error {
	dir := l.ModelDir(namespace, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, "lock.json")
	if _, err := os.Stat(path); err == nil {
		return ErrLocked
	}
	b, _ := json.Marshal(lockFile{Owner: owner, Acquired: time.Now().UTC()})
	return os.WriteFile(path, b, 0o644)
}

// ReleaseModelLock removes the lock.json for a model cache entry.
func (l *Layout) ReleaseModelLock(namespace, name string) {
	_ = os.Remove(filepath.Join(l.ModelDir(namespace, name), "lock.json"))
}
