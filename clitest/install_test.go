package clitest_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"

	"github.com/circleci-functions/toolkit/sdk/sdktest"

	"github.com/circleci-functions/setup-ruby/internal/testing/fakedist"
	"github.com/circleci-functions/setup-ruby/internal/testing/fakerbenv"
	"github.com/circleci-functions/setup-ruby/rubydist"
)

var releases = []fakedist.Release{
	{Version: "3.3.6"},
	{Version: "3.4.0"},
	{Version: "3.4.1"},
	{Version: "3.5.0-preview1"},
}

// installEnv starts a fake download server and returns it with a sandbox and
// a bin dir to link into. The fake builds' executables are shell scripts, so
// these tests need a Unix.
func installEnv(t *testing.T) (*fakedist.Server, *sdktest.Sandbox, string) {
	t.Helper()
	skip.If(t, runtime.GOOS == "windows", "the fake Ruby builds are shell scripts")
	return fakedist.New(t, releases...), sdktest.NewSandbox(t), t.TempDir()
}

// installArgs are the flags that point an install at srv and binDir.
func installArgs(srv *fakedist.Server, binDir string, args ...string) []string {
	return append(args, "--downloads-url", srv.CatalogURL(), "--bin-dir", binDir)
}

// noRuby is a PATH with no ruby on it.
func noRuby(t *testing.T) map[string]string {
	t.Helper()
	return map[string]string{"PATH": t.TempDir()}
}

func rubyVersion(t *testing.T, binDir string) string {
	t.Helper()
	out, err := exec.Command(filepath.Join(binDir, "ruby"), "-v").Output()
	assert.NilError(t, err)
	fields := strings.Fields(string(out))
	assert.Assert(t, len(fields) > 1, "ruby -v: %s", out)
	return fields[1]
}

func installRoot(t *testing.T, s *sdktest.Sandbox, version string) string {
	t.Helper()
	p, err := rubydist.Host()
	assert.NilError(t, err)
	return filepath.Join(s.Home, ".cache", "setup-ruby", version+"-"+p.String())
}

func TestCLI_Install_ResolvesVersion(t *testing.T) {
	dir := t.TempDir()
	assert.NilError(t, os.WriteFile(filepath.Join(dir, ".tool-versions"), []byte("ruby 3.3.6\n"), 0600))

	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "minor", args: []string{"--ruby-version", "3.4"}, want: "3.4.1"},
		{name: "exact", args: []string{"--ruby-version", "3.4.0"}, want: "3.4.0"},
		{name: "latest", args: []string{"--ruby-version", "latest"}, want: "3.4.1"},
		{name: "prerelease fallback", args: []string{"--ruby-version", "3.5"}, want: "3.5.0preview1"},
		{name: "version file", args: []string{"--ruby-version-file", filepath.Join(dir, ".tool-versions")}, want: "3.3.6"},
		{name: "explicit install command", args: []string{"install", "--ruby-version", "3.3"}, want: "3.3.6"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv, s, binDir := installEnv(t)

			out, err := runIn(t, s, "", nil, installArgs(srv, binDir, tc.args...)...)
			assert.NilError(t, err, "output: %s", out)
			assert.Equal(t, rubyVersion(t, binDir), tc.want)
		})
	}
}

func TestCLI_Install_VersionFlagWinsOverFile(t *testing.T) {
	srv, s, binDir := installEnv(t)
	file := filepath.Join(t.TempDir(), ".ruby-version")
	assert.NilError(t, os.WriteFile(file, []byte("3.3\n"), 0600))

	out, err := runIn(t, s, "", nil, installArgs(srv, binDir, "--ruby-version", "3.4", "--ruby-version-file", file)...)
	assert.NilError(t, err, "output: %s", out)
	assert.Equal(t, rubyVersion(t, binDir), "3.4.1")
	assert.Assert(t, strings.Contains(out, "using --ruby-version"), "expected a warning; got: %s", out)
}

func TestCLI_Install_DefaultVersion(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
		want  string
	}{
		{name: "ruby-version", files: map[string]string{".ruby-version": "ruby-3.3.6\n"}, want: "3.3.6"},
		{name: "tool-versions", files: map[string]string{".tool-versions": "ruby 3.4.0\n"}, want: "3.4.0"},
		{name: "mise", files: map[string]string{"mise.toml": "[tools]\nruby = \"3.3\"\n"}, want: "3.3.6"},
		{name: "newest stable without a Ruby on PATH", want: "3.4.1"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv, s, binDir := installEnv(t)
			dir := t.TempDir()
			for name, content := range tc.files {
				assert.NilError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0600))
			}

			out, err := runIn(t, s, "", noRuby(t), installArgs(srv, binDir, "--working-directory", dir)...)
			assert.NilError(t, err, "output: %s", out)
			assert.Equal(t, rubyVersion(t, binDir), tc.want)
		})
	}
}

func TestCLI_Install_WorkingDirectoryDefaultsToTheStepsOwn(t *testing.T) {
	srv, s, binDir := installEnv(t)
	dir := t.TempDir()
	assert.NilError(t, os.WriteFile(filepath.Join(dir, ".ruby-version"), []byte("3.4.0\n"), 0600))

	out, err := runIn(t, s, dir, noRuby(t), installArgs(srv, binDir)...)
	assert.NilError(t, err, "output: %s", out)
	assert.Equal(t, rubyVersion(t, binDir), "3.4.0")
}

// TestCLI_Install_UsesTheRubyOnPath checks that with no version asked for, a
// Ruby already on PATH is used as it is, and nothing is downloaded.
func TestCLI_Install_UsesTheRubyOnPath(t *testing.T) {
	srv, s, binDir := installEnv(t)
	pathDir := t.TempDir()
	//nolint:gosec // test stub; 0700 so it is executable
	assert.NilError(t, os.WriteFile(filepath.Join(pathDir, "ruby"), []byte("#!/bin/sh\necho 'ruby 3.2.2 (2023-03-30 revision e51014f9c0) [x86_64-linux]'\n"), 0700))

	out, err := runIn(t, s, t.TempDir(), map[string]string{"PATH": pathDir}, installArgs(srv, binDir)...)
	assert.NilError(t, err, "output: %s", out)
	assert.Assert(t, strings.Contains(out, "using ruby 3.2.2"), "output: %s", out)
	assert.Equal(t, srv.Downloads(), 0)
	entries, err := os.ReadDir(binDir)
	assert.NilError(t, err)
	assert.Equal(t, len(entries), 0, "nothing should be linked into --bin-dir: %v", entries)
}

func TestCLI_Install_LinksTools(t *testing.T) {
	srv, s, binDir := installEnv(t)

	out, err := runIn(t, s, "", nil, installArgs(srv, binDir, "--ruby-version", "3.4")...)
	assert.NilError(t, err, "output: %s", out)
	for _, tool := range []string{"ruby", "gem", "bundle", "bundler", "irb", "rake", "erb", "rdoc", "ri"} {
		_, err := os.Stat(filepath.Join(binDir, tool))
		assert.NilError(t, err, "%s not linked", tool)
	}
}

func TestCLI_Install_GemExecutablesLandBesideRuby(t *testing.T) {
	srv, s, binDir := installEnv(t)

	out, err := runIn(t, s, "", nil, installArgs(srv, binDir, "--ruby-version", "3.4")...)
	assert.NilError(t, err, "output: %s", out)
	assert.Assert(t, strings.Contains(out, "executables from gem install land in "+binDir), "output: %s", out)
	hook, err := os.ReadFile(filepath.Join(installRoot(t, s, "3.4.1"), "lib", "ruby", "site_ruby", "3.4.0", "rubygems", "defaults", "operating_system.rb"))
	assert.NilError(t, err)
	assert.Assert(t, strings.Contains(string(hook), "'"+binDir+"'"), "hook: %s", hook)
}

func TestCLI_Install_WritesGemrc(t *testing.T) {
	srv, s, binDir := installEnv(t)

	out, err := runIn(t, s, "", nil, installArgs(srv, binDir, "--ruby-version", "3.4")...)
	assert.NilError(t, err, "output: %s", out)
	data, err := os.ReadFile(filepath.Join(s.Home, ".gemrc"))
	assert.NilError(t, err)
	assert.Equal(t, string(data), "gem: --no-document\n")

	assert.NilError(t, os.WriteFile(filepath.Join(s.Home, ".gemrc"), []byte("mine\n"), 0600))
	out, err = runIn(t, s, "", nil, installArgs(srv, binDir, "--ruby-version", "3.4")...)
	assert.NilError(t, err, "output: %s", out)
	data, err = os.ReadFile(filepath.Join(s.Home, ".gemrc"))
	assert.NilError(t, err)
	assert.Equal(t, string(data), "mine\n", "an existing ~/.gemrc must be left alone")
}

// TestCLI_Install_RubygemsAndBundler checks the gem commands --rubygems and
// --bundler run, through the fake build's gem, which logs how it is run and
// reports RubyGems 3.6.9.
func TestCLI_Install_RubygemsAndBundler(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		lockfile string
		want     []string
	}{
		{name: "defaults without a lock file"},
		{name: "rubygems latest", args: []string{"--rubygems", "latest"}, want: []string{"gem update --system"}},
		{name: "rubygems newer", args: []string{"--rubygems", "3.7.2"}, want: []string{"gem update --system 3.7.2"}},
		{name: "rubygems older", args: []string{"--rubygems", "3.5.0"}},
		{
			name:     "bundler from the lock file",
			lockfile: "GEM\n  specs:\n\nBUNDLED WITH\n   2.6.9\n",
			want:     []string{"gem list --installed bundler --version 2.6.9", "gem install bundler -v 2.6.9"},
		},
		{name: "lock file without bundled with", lockfile: "GEM\n  specs:\n"},
		{name: "bundler partial", args: []string{"--bundler", "2.6"}, want: []string{"gem list --installed bundler --version ~> 2.6.0", "gem install bundler -v ~> 2.6.0"}},
		{name: "bundler latest", args: []string{"--bundler", "latest"}, want: []string{"gem install bundler"}},
		{name: "bundler none", args: []string{"--bundler", "none"}, lockfile: "BUNDLED WITH\n   2.6.9\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv, s, binDir := installEnv(t)
			dir := t.TempDir()
			if tc.lockfile != "" {
				assert.NilError(t, os.WriteFile(filepath.Join(dir, "Gemfile"), nil, 0600))
				assert.NilError(t, os.WriteFile(filepath.Join(dir, "Gemfile.lock"), []byte(tc.lockfile), 0600))
			}
			logPath := filepath.Join(t.TempDir(), "stub.log")

			args := append([]string{"--ruby-version", "3.4", "--working-directory", dir}, tc.args...)
			out, err := runIn(t, s, "", map[string]string{"STUB_LOG": logPath}, installArgs(srv, binDir, args...)...)
			assert.NilError(t, err, "output: %s", out)
			assert.DeepEqual(t, stubCalls(t, logPath), tc.want)
		})
	}
}

// stubCalls returns the invocations the fake build's executables logged, one
// per line.
func stubCalls(t *testing.T, logPath string) []string {
	t.Helper()
	data, err := os.ReadFile(logPath) //nolint:gosec // a test temp file
	if os.IsNotExist(err) {
		return nil
	}
	assert.NilError(t, err)
	return strings.Split(strings.TrimSpace(string(data)), "\n")
}

func TestCLI_Install_Cache(t *testing.T) {
	p, err := rubydist.Host()
	assert.NilError(t, err)
	tests := []struct {
		name       string
		args       []string
		wantSuffix string
	}{
		{name: "default"},
		{name: "nonce", args: []string{"--cache-nonce", "run-42"}, wantSuffix: "-run-42"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv, s, binDir := installEnv(t)
			rec, pluginURL := newCacheRecorder(t)

			args := append([]string{"--ruby-version", "3.4"}, tc.args...)
			out, err := runIn(t, s, "", map[string]string{"CIRCLECI_PLUGIN_URL": pluginURL}, installArgs(srv, binDir, args...)...)
			assert.NilError(t, err, "output: %s", out)

			assert.Equal(t, len(rec.saves), 1)
			key := rec.saves[0].Key
			prefix := "setup-ruby-v1-3.4.1-" + p.String() + "-" + srv.Build(fakedist.Release{Version: "3.4.1"}).SHA256[:8]
			assert.Assert(t, cmp.Regexp(`^`+regexp.QuoteMeta(prefix)+`-[0-9a-f]{8}`+regexp.QuoteMeta(tc.wantSuffix)+`$`, key))
			assert.DeepEqual(t, rec.saves[0].Paths, []string{installRoot(t, s, "3.4.1")})
			assert.Equal(t, len(rec.restores), 1)
			assert.DeepEqual(t, rec.restores[0], []string{key})
		})
	}
}

// TestCLI_Install_CacheKeyDependsOnWhereTheInstallLives covers executors whose
// $HOME differs: the cache restores to the path it saved, so they must not
// share an entry.
func TestCLI_Install_CacheKeyDependsOnWhereTheInstallLives(t *testing.T) {
	keys := map[string]bool{}
	for range 2 {
		srv, s, binDir := installEnv(t)
		rec, pluginURL := newCacheRecorder(t)

		out, err := runIn(t, s, "", map[string]string{"CIRCLECI_PLUGIN_URL": pluginURL}, installArgs(srv, binDir, "--ruby-version", "3.4")...)
		assert.NilError(t, err, "output: %s", out)
		assert.Equal(t, len(rec.saves), 1)
		keys[rec.saves[0].Key] = true
	}
	assert.Equal(t, len(keys), 2, "two homes shared a cache key: %v", keys)
}

func TestCLI_Install_CacheDisabled(t *testing.T) {
	srv, s, binDir := installEnv(t)
	rec, pluginURL := newCacheRecorder(t)

	out, err := runIn(t, s, "", map[string]string{"CIRCLECI_PLUGIN_URL": pluginURL}, installArgs(srv, binDir, "--ruby-version", "3.4", "--cache=false")...)
	assert.NilError(t, err, "output: %s", out)
	assert.Equal(t, len(rec.restores), 0)
	assert.Equal(t, len(rec.saves), 0)
	assert.Equal(t, srv.Downloads(), 1)
}

func TestCLI_Install_RestoresFromCache(t *testing.T) {
	srv, s, binDir := installEnv(t)
	args := installArgs(srv, binDir, "--ruby-version", "3.4")

	out, err := runIn(t, s, "", nil, args...)
	assert.NilError(t, err, "output: %s", out)
	assert.Equal(t, srv.Downloads(), 1)

	assert.NilError(t, os.RemoveAll(filepath.Join(s.Home, ".cache", "setup-ruby")))
	out, err = runIn(t, s, "", nil, args...)
	assert.NilError(t, err, "output: %s", out)
	assert.Equal(t, srv.Downloads(), 1, "expected the second run to restore rather than download")
	assert.Assert(t, strings.Contains(out, "restored Ruby 3.4.1 from cache"), "output: %s", out)
	assert.Equal(t, rubyVersion(t, binDir), "3.4.1")
}

func TestCLI_Install_RerunReusesInstall(t *testing.T) {
	srv, s, binDir := installEnv(t)
	rec, pluginURL := newCacheRecorder(t)
	env := map[string]string{"CIRCLECI_PLUGIN_URL": pluginURL}
	args := installArgs(srv, binDir, "--ruby-version", "3.4")

	for range 2 {
		out, err := runIn(t, s, "", env, args...)
		assert.NilError(t, err, "output: %s", out)
	}
	assert.Equal(t, srv.Downloads(), 1)
	assert.Equal(t, len(rec.restores), 1, "a rerun should not touch the cache")
}

// TestCLI_Install_RebuildReplacesInstall covers a version republished with a
// fixed build: the install an earlier job cached may not pass for the new
// build, and the new build must be cached in turn.
func TestCLI_Install_RebuildReplacesInstall(t *testing.T) {
	first, s, binDir := installEnv(t)
	out, err := runIn(t, s, "", nil, installArgs(first, binDir, "--ruby-version", "3.4.1")...)
	assert.NilError(t, err, "output: %s", out)
	assert.NilError(t, os.RemoveAll(filepath.Join(s.Home, ".cache", "setup-ruby")))

	second := fakedist.New(t, fakedist.Release{Version: "3.4.1", Rebuild: 1})
	out, err = runIn(t, s, "", nil, installArgs(second, binDir, "--ruby-version", "3.4.1")...)
	assert.NilError(t, err, "output: %s", out)
	assert.Equal(t, second.Downloads(), 1, "expected the rebuild to be downloaded; output: %s", out)
	_, err = os.Stat(filepath.Join(installRoot(t, s, "3.4.1"), "REBUILD"))
	assert.NilError(t, err, "expected the rebuild to be installed")

	assert.NilError(t, os.RemoveAll(filepath.Join(s.Home, ".cache", "setup-ruby")))
	out, err = runIn(t, s, "", nil, installArgs(second, binDir, "--ruby-version", "3.4.1")...)
	assert.NilError(t, err, "output: %s", out)
	assert.Equal(t, second.Downloads(), 1, "expected the rebuild to be cached; output: %s", out)
}

// TestCLI_Install_RbenvPartialVersionFile covers a .ruby-version naming a
// partial version, which rbenv looks up by that name only.
func TestCLI_Install_RbenvPartialVersionFile(t *testing.T) {
	srv, s, _ := installEnv(t)
	rbenv := fakerbenv.New(t, "3.3.0")
	app := filepath.Join(t.TempDir(), "app")
	assert.NilError(t, os.MkdirAll(app, 0750))
	assert.NilError(t, os.WriteFile(filepath.Join(app, ".ruby-version"), []byte("3.3\n"), 0600))
	path := strings.Join([]string{filepath.Join(rbenv, "shims"), filepath.Join(rbenv, "bin"), "/usr/bin", "/bin"}, string(os.PathListSeparator))

	out, err := runIn(t, s, t.TempDir(), map[string]string{"PATH": path}, "--downloads-url", srv.CatalogURL(), "--working-directory", app)
	assert.NilError(t, err, "output: %s", out)

	target, err := os.Readlink(filepath.Join(rbenv, "versions", "3.3"))
	assert.NilError(t, err)
	assert.Equal(t, target, installRoot(t, s, "3.3.6"))
	cmd := exec.Command(filepath.Join(rbenv, "shims", "ruby"), "-v")
	cmd.Dir = app
	got, err := cmd.Output()
	assert.NilError(t, err)
	assert.Assert(t, strings.HasPrefix(string(got), "ruby 3.3.6 "), "ruby -v in %s: %s", app, got)
}

func TestCLI_Install_Errors(t *testing.T) {
	p, err := rubydist.Host()
	assert.NilError(t, err)
	tests := []struct {
		name    string
		args    []string
		corrupt bool
		wantErr string
	}{
		{name: "no matching version", args: []string{"--ruby-version", "3.2"}, wantErr: `no Ruby release matches 3.2 for ` + p.String()},
		{name: "invalid version", args: []string{"--ruby-version", "banana"}, wantErr: `invalid ruby version "banana"`},
		{name: "jruby", args: []string{"--ruby-version", "jruby-9.4.8.0"}, wantErr: "only CRuby is supported, not jruby"},
		{name: "head", args: []string{"--ruby-version", "head"}, wantErr: "development builds are not supported"},
		{name: "missing version file", args: []string{"--ruby-version-file", "/nonexistent/.ruby-version"}, wantErr: "ruby-version-file: open /nonexistent/.ruby-version: no such file or directory"},
		{name: "invalid windows toolchain", args: []string{"--ruby-version", "3.4", "--windows-toolchain", "vcpkg"}, wantErr: "--windows-toolchain must be one of"},
		{name: "checksum mismatch", args: []string{"--ruby-version", "3.4"}, corrupt: true, wantErr: "checksum mismatch"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv, s, binDir := installEnv(t)
			if tc.corrupt {
				srv.Corrupt()
			}

			out, err := runIn(t, s, "", nil, installArgs(srv, binDir, tc.args...)...)
			assert.Assert(t, err != nil, "expected a non-zero exit; output: %s", out)
			assert.Assert(t, strings.Contains(out, tc.wantErr), "expected %q in output: %s", tc.wantErr, out)
		})
	}
}
