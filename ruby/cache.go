package ruby

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
)

// LockfileInput describes how to locate the dependency lock file used for
// cache keying.
type LockfileInput struct {
	// Dir is the directory containing the Gemfile.
	Dir string
	// Gemfile is the Gemfile name, e.g. "Gemfile" or a ".rb" file.
	Gemfile string
	// Override, when set, names a lock file to use instead of the detected one.
	Override string
	// DepsNext reports whether the DEPENDENCIES_NEXT environment variable is set.
	DepsNext bool
}

// ResolveLockfile returns the path to the lock file to hash for the cache key,
// following the orb's resolution order: an explicit override, then
// Gemfile_next.lock when DepsNext is set, then a ".locked" sibling for a ".rb"
// gemfile, then "<gemfile>.lock". It returns ok=false when none exists.
func ResolveLockfile(in LockfileInput) (path string, ok bool) {
	resolve := func(name string) string {
		if filepath.IsAbs(name) {
			return name
		}
		return filepath.Join(in.Dir, name)
	}

	if in.Override != "" {
		if p := resolve(in.Override); exists(p) {
			return p, true
		}
	}

	if in.DepsNext {
		if p := resolve("Gemfile_next.lock"); exists(p) {
			return p, true
		}
	}

	if filepath.Ext(in.Gemfile) == ".rb" {
		locked := in.Gemfile[:len(in.Gemfile)-len(".rb")] + ".locked"
		if p := resolve(locked); exists(p) {
			return p, true
		}
		return "", false
	}

	if p := resolve(in.Gemfile + ".lock"); exists(p) {
		return p, true
	}

	return "", false
}

// Checksum returns the hex-encoded SHA-256 of the file at path.
func Checksum(path string) (string, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path is a resolved lockfile
	if err != nil {
		return "", fmt.Errorf("read lockfile %s: %w", path, err)
	}
	return fmt.Sprintf("%x", sha256.Sum256(data)), nil
}

// CacheKeyParts are the pieces that make up a gem cache key.
type CacheKeyParts struct {
	// Prefix is the user-facing cache key, e.g. "gems-v1".
	Prefix string
	// Arch is included as a key segment when non-empty.
	Arch string
	// Branch is included as a key segment when non-empty.
	Branch string
	// Checksum is the lock file checksum forming the exact key.
	Checksum string
}

func (p CacheKeyParts) base() string {
	b := p.Prefix + "-"
	if p.Arch != "" {
		b += p.Arch + "-"
	}
	if p.Branch != "" {
		b += p.Branch + "-"
	}
	return b
}

// Exact returns the fully-qualified key including the checksum.
func (p CacheKeyParts) Exact() string {
	return p.base() + p.Checksum
}

// RestoreKeys returns the keys to try when restoring, most specific first. When
// onlyChecksum is true only the exact key is returned; otherwise a loose prefix
// key (without the checksum) is appended as a fallback.
func (p CacheKeyParts) RestoreKeys(onlyChecksum bool) []string {
	if onlyChecksum {
		return []string{p.Exact()}
	}
	return []string{p.Exact(), p.base()}
}

func exists(path string) bool {
	info, err := os.Stat(path) //nolint:gosec // path is a lock file or known directory
	return err == nil && !info.IsDir()
}

func isDir(path string) bool {
	info, err := os.Stat(path) //nolint:gosec // path is a known directory
	return err == nil && info.IsDir()
}
