package offsetstore

import "time"

// OffsetStore defines the interface for loading and saving offsets.
type OffsetStore interface {
    LoadOffset(path string, fallbackDuration time.Duration) int64
    SaveOffset(path string, offset int64) error
}

