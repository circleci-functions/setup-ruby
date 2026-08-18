package setupruby_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/circleci-functions/toolkit/sdk"
)

type cacheSave struct {
	Key   string
	Paths []string
}

type cacheRecorder struct {
	mu       sync.Mutex
	restores [][]string
	saves    []cacheSave
}

func newCacheRecorder(t *testing.T) (*cacheRecorder, string) {
	t.Helper()
	rec := &cacheRecorder{}

	mux := http.NewServeMux()
	mux.HandleFunc("/step/cache/restore", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Keys []string `json:"keys"`
		}
		assert.NilError(t, json.NewDecoder(r.Body).Decode(&req))
		rec.mu.Lock()
		rec.restores = append(rec.restores, req.Keys)
		rec.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"result": "success"})
	})
	mux.HandleFunc("/step/cache/save", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Key   string   `json:"key"`
			Paths []string `json:"paths"`
		}
		assert.NilError(t, json.NewDecoder(r.Body).Decode(&req))
		rec.mu.Lock()
		rec.saves = append(rec.saves, cacheSave{Key: req.Key, Paths: req.Paths})
		rec.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"result": "success"})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return rec, srv.URL
}

func installDepsEnv(t *testing.T, pluginURL string) map[string]string {
	t.Helper()
	return map[string]string{
		"CIRCLECI_PLUGIN_URL": pluginURL,
		"PATH":                fakeRubyToolsPath(t) + string(os.PathListSeparator) + os.Getenv("PATH"),
		"CIRCLE_BRANCH":       "main",
	}
}

func TestInstallDeps_CacheSaveAndRestore(t *testing.T) {
	rec, url := newCacheRecorder(t)
	appDir := t.TempDir()
	copyFixture(t, appDir)

	out, err := run(t, installDepsEnv(t, url), "install-deps", "--app-dir", appDir)
	assert.NilError(t, err, "output: %s", out)

	assert.Equal(t, len(rec.restores), 1)
	assert.Equal(t, len(rec.restores[0]), 2, "expected exact key plus loose fallback")
	exact := rec.restores[0][0]
	assert.Assert(t, strings.HasPrefix(exact, "gems-v1-"), "unexpected key: %s", exact)
	assert.Assert(t, strings.Contains(exact, sdk.OSArch()), "key should include arch: %s", exact)
	assert.Assert(t, strings.Contains(exact, "-main-"), "key should include branch: %s", exact)
	prefix := exact[:strings.LastIndex(exact, "-")+1]
	assert.Equal(t, rec.restores[0][1], prefix, "fallback key must be the exact key without the checksum")

	assert.Equal(t, len(rec.saves), 1)
	assert.Equal(t, rec.saves[0].Key, exact, "save key must match restore's exact key")
	assert.DeepEqual(t, rec.saves[0].Paths, []string{filepath.Join(appDir, "vendor", "bundle")})
}

func TestInstallDeps_NoWithCache(t *testing.T) {
	rec, url := newCacheRecorder(t)
	appDir := t.TempDir()
	copyFixture(t, appDir)

	out, err := run(t, installDepsEnv(t, url), "install-deps", "--app-dir", appDir, "--no-with-cache")
	assert.NilError(t, err, "output: %s", out)

	assert.Equal(t, len(rec.restores), 0)
	assert.Equal(t, len(rec.saves), 0)
}

func TestInstallDeps_OnlyChecksumKey(t *testing.T) {
	rec, url := newCacheRecorder(t)
	appDir := t.TempDir()
	copyFixture(t, appDir)

	out, err := run(t, installDepsEnv(t, url), "install-deps", "--app-dir", appDir, "--only-use-gemfile-checksum-cache-key")
	assert.NilError(t, err, "output: %s", out)

	assert.Equal(t, len(rec.restores), 1)
	assert.Equal(t, len(rec.restores[0]), 1, "only-checksum drops the loose fallback key")
}

func TestInstallDeps_NoArchNoBranchInKey(t *testing.T) {
	rec, url := newCacheRecorder(t)
	appDir := t.TempDir()
	copyFixture(t, appDir)

	out, err := run(t, installDepsEnv(t, url), "install-deps", "--app-dir", appDir,
		"--no-include-arch-in-cache-key", "--no-include-branch-in-cache-key")
	assert.NilError(t, err, "output: %s", out)

	exact := rec.restores[0][0]
	assert.Assert(t, !strings.Contains(exact, sdk.OSArch()), "key should omit arch: %s", exact)
	assert.Assert(t, !strings.Contains(exact, "-main-"), "key should omit branch: %s", exact)
	assert.Assert(t, strings.HasPrefix(exact, "gems-v1-"), "unexpected key: %s", exact)
}

func TestInstallDeps_KeyChangesWithLockfile(t *testing.T) {
	rec, url := newCacheRecorder(t)

	keyFor := func(suffix string) string {
		appDir := t.TempDir()
		copyFixture(t, appDir)
		lock := filepath.Join(appDir, "Gemfile.lock")
		data, rerr := os.ReadFile(lock) //nolint:gosec // fixture path
		assert.NilError(t, rerr)
		assert.NilError(t, os.WriteFile(lock, append(data, []byte(suffix)...), 0o600))
		out, err := run(t, installDepsEnv(t, url), "install-deps", "--app-dir", appDir)
		assert.NilError(t, err, "output: %s", out)
		return rec.restores[len(rec.restores)-1][0]
	}

	assert.Assert(t, keyFor("") != keyFor("\n# changed"), "key should change with lockfile content")
}

func TestInstallDeps_NoLockfileSkipsCache(t *testing.T) {
	rec, url := newCacheRecorder(t)
	appDir := t.TempDir()
	assert.NilError(t, os.WriteFile(filepath.Join(appDir, "Gemfile"), []byte("source 'https://rubygems.org'\n"), 0o600))

	out, err := run(t, installDepsEnv(t, url), "install-deps", "--app-dir", appDir)
	assert.NilError(t, err, "output: %s", out)
	assert.Assert(t, strings.Contains(out, "skipping cache"), "expected skip message; got: %s", out)
	assert.Equal(t, len(rec.restores), 0)
	assert.Equal(t, len(rec.saves), 0)
}
