package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/charmbracelet/log"
)

// LoadResult contains the loaded config and metadata about the load.
type LoadResult struct {
	Config      Config
	SourcePaths []string // paths that were successfully loaded, in order applied
}

// FileSystem abstracts file system operations for testability.
type FileSystem interface {
	// Exists returns true if the path exists and is a file (not a directory).
	Exists(path string) bool
}

// OSFileSystem implements FileSystem using the real OS.
type OSFileSystem struct{}

// Exists returns true if the path exists and is a file (not a directory).
func (OSFileSystem) Exists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// Loader handles configuration loading and merging.
type Loader struct {
	fs FileSystem
}

// NewLoader creates a new Loader with the given FileSystem.
func NewLoader(fs FileSystem) *Loader {
	return &Loader{fs: fs}
}

// NewDefaultLoader creates a new Loader that uses the real OS file system.
func NewDefaultLoader() *Loader {
	return NewLoader(OSFileSystem{})
}

// Load reads and merges all config files in priority order.
// Paths should be ordered from lowest to highest priority.
// Returns merged config with defaults as base, plus source paths for debugging.
func (l *Loader) Load(paths []string) (LoadResult, error) {
	cfg := DefaultConfig()
	var sourcePaths []string

	for _, path := range paths {
		if !l.fs.Exists(path) {
			continue // Skip missing files
		}

		metadata, err := toml.DecodeFile(path, &cfg)
		if err != nil {
			return LoadResult{}, fmt.Errorf("failed to parse %s: %w", path, err)
		}

		if undecoded := metadata.Undecoded(); len(undecoded) > 0 {
			log.Warn("unknown config keys", "path", path, "keys", undecoded)
		}

		sourcePaths = append(sourcePaths, path)
	}

	if err := cfg.Validate(); err != nil {
		return LoadResult{}, fmt.Errorf("invalid config: %w", err)
	}

	return LoadResult{
		Config:      cfg,
		SourcePaths: sourcePaths,
	}, nil
}
