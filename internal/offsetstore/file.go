package offsetstore

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"cwsync/internal/logger"
)

type FileOffsetStore struct {
	Path   string
	mu     sync.Mutex
	data   map[string]int64
	loaded bool
}

func NewFileOffsetStore(path string) *FileOffsetStore {
	return &FileOffsetStore{Path: path, data: map[string]int64{}}
}

func (f *FileOffsetStore) LoadOffset(key string, fallbackDuration time.Duration) int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.ensureLoaded(); err != nil {
		logger.Error.Printf("FileOffsetStore load failed, using fallback: %v", err)
		return time.Now().UTC().Add(-fallbackDuration).UnixMilli()
	}
	if v, ok := f.data[key]; ok {
		return v
	}
	return time.Now().UTC().Add(-fallbackDuration).UnixMilli()
}

func (f *FileOffsetStore) SaveOffset(key string, offset int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.ensureLoaded(); err != nil {
		logger.Error.Printf("FileOffsetStore ensureLoaded failed: %v", err)
	}
	f.data[key] = offset
	if err := os.MkdirAll(filepath.Dir(f.Path), 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(f.Path), "offsetstore-*.tmp")
	if err != nil {
		return err
	}
	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	if err := enc.Encode(f.data); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	_ = tmp.Chmod(0600)
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), f.Path); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return nil
}

func (f *FileOffsetStore) ensureLoaded() error {
	if f.loaded {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(f.Path), 0755); err != nil {
		return err
	}
	if _, err := os.Stat(f.Path); os.IsNotExist(err) {
		if err := os.WriteFile(f.Path, []byte("{}\n"), 0600); err != nil {
			return err
		}
		f.data = map[string]int64{}
		f.loaded = true
		return nil
	}
	b, err := os.ReadFile(f.Path)
	if err != nil {
		return err
	}
	m := map[string]int64{}
	if err := json.Unmarshal(b, &m); err != nil {
		bak := f.Path + ".bak." + strconv.FormatInt(time.Now().Unix(), 10)
		if rerr := os.Rename(f.Path, bak); rerr != nil {
			logger.Error.Printf("Failed to backup corrupt offset file: %v", rerr)
		}
		f.data = map[string]int64{}
	} else {
		f.data = m
	}
	f.loaded = true
	return nil
}
