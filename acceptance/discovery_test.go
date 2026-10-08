//go:build acceptance

package setupruby_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/skip"

	"github.com/circleci-functions/toolkit/sdk"
	"github.com/circleci-functions/toolkit/sdk/sdktest"
	"github.com/circleci-functions/toolkit/testing/runner"
)

// TestAcceptance_InstallsOntoTheRealPath is the one test that lets setup-ruby
// choose where ruby is linked, rather than telling it with --bin-dir. That
// choice depends on the executor, and a sandboxed suite never reaches it.
//
// It changes the machine it runs on, up to taking over a Ruby already there,
// so it runs only where that is disposable: a CircleCI job, or a container
// the caller vouches for with $SETUP_RUBY_DISPOSABLE (see `task smoke-local`).
func TestAcceptance_InstallsOntoTheRealPath(t *testing.T) {
	if os.Getenv("SETUP_RUBY_DISPOSABLE") == "" {
		sdktest.SkipUnlessInJob(t)
	}

	s := sdktest.NewSandbox(t, sdktest.WithLiveCache())
	spec := acceptanceVersion()

	run := func(t *testing.T, dir string, args ...string) string {
		t.Helper()
		opts := []runner.Opt{runner.WithUnsafeOSEnvVars()}
		if dir != "" {
			opts = append(opts, runner.WithCmdDir(dir))
		}
		r := runner.New(t, binaryPath, opts...)
		_, err := r.Run(args...)
		out := r.StdoutString() + r.StderrString()
		assert.NilError(t, err, "got: %s", out)
		return out
	}

	installArgs := []string{
		"--ruby-version", spec,
		"--install-dir", s.InstallDir,
		"--cache-nonce", s.CacheKeyNonce,
	}
	if path, err := exec.LookPath("ruby"); err == nil {
		t.Logf("ruby on PATH before setup-ruby: %s", path)
	}
	out := run(t, "", installArgs...)
	t.Logf("setup-ruby: %s", out)
	assert.Assert(t, !strings.Contains(out, "from cache"),
		"a per-run nonce must miss the cache, so this install downloads for real; got: %s", out)

	version := reported(t, out, `Ruby (\S+) installed at`)
	if exactVersion.MatchString(spec) {
		assert.Equal(t, version, spec)
	}
	linked := reported(t, out, `linked into (.+)$`)
	t.Logf("setup-ruby installed Ruby %s and linked it into %s", version, linked)
	rubyV := "ruby " + strings.ReplaceAll(version, "-", "")

	t.Run("ResolvesByNameInALaterStep", func(t *testing.T) {
		got, err := sdktest.LaterStep(t, "ruby", "-v")
		assert.NilError(t, err, "a later step could not resolve ruby: %s", got)
		assert.Assert(t, strings.HasPrefix(strings.TrimSpace(got), rubyV), "ruby -v: %s", got)
		for _, name := range []string{"gem", "bundle"} {
			got, err := sdktest.LaterStep(t, name, "--version")
			assert.NilError(t, err, "a later step could not resolve %s: %s", name, got)
		}
		got, err = sdktest.LaterStep(t, "gem", "env")
		assert.NilError(t, err, "gem env: %s", got)
		gemRuby := find(got, `RUBY EXECUTABLE: (.+)$`)
		assert.Assert(t, isWithin(gemRuby, s.InstallDir), "gem belongs to another Ruby: %s", got)
	})

	if runtime.GOOS == "windows" {
		t.Run("ResolvesInPowerShellToo", func(t *testing.T) {
			got, err := sdktest.LaterStep(t, "powershell.exe", "-Command", "ruby -v; gem --version; bundle --version")
			assert.NilError(t, err, "PowerShell could not resolve ruby, gem and bundle: %s", got)
			assert.Assert(t, strings.Contains(got, rubyV), "PowerShell resolved a different ruby: %s", got)
		})
	}

	t.Run("GemInstallsResolveInALaterStepWhereTheyCan", func(t *testing.T) {
		got, err := sdktest.LaterStep(t, "gem", "install", "dotenv", "-v", "3.1.8")
		assert.NilError(t, err, "gem install: %s", got)
		skip.If(t, strings.Contains(out, "which is not on PATH"), "gem install's executables are not on PATH on this executor")
		got, err = sdktest.LaterStep(t, "dotenv", "--version")
		assert.NilError(t, err, "a later step could not run what gem install installed: %s", got)
		assert.Assert(t, strings.Contains(got, "3.1.8"), "dotenv --version: %s", got)
	})

	t.Run("CompilesNativeExtensions", func(t *testing.T) {
		if runtime.GOOS != "windows" {
			_, err := exec.LookPath("cc")
			skip.If(t, err != nil, "no C compiler on this executor")
		}
		got, err := sdktest.LaterStep(t, "gem", "install", "bigdecimal", "-v", "3.1.9", "--platform", "ruby")
		assert.NilError(t, err, "gem install bigdecimal: %s", got)
		got, err = sdktest.LaterStep(t, "ruby", "-e", "gem 'bigdecimal', '3.1.9'; require 'bigdecimal'; puts BigDecimal('1.5') + 1")
		assert.NilError(t, err, "the compiled extension does not load: %s", got)
		assert.Equal(t, strings.TrimSpace(got), "0.25e1")
	})

	t.Run("InstallsGemsWithTheInstalledRuby", func(t *testing.T) {
		app := writeApp(t)
		got, err := laterStepIn(t, app, "bundle", "lock")
		assert.NilError(t, err, "bundle lock: %s", got)
		out := run(t, app, "install-gems", "--key", "gems-"+s.CacheKeyNonce)
		t.Logf("install-gems: %s", out)
		got, err = laterStepIn(t, app, "bundle", "exec", "rake", "--version")
		assert.NilError(t, err, "bundle exec rake: %s", got)
		assert.Assert(t, strings.Contains(got, "13.2.1"), "rake --version: %s", got)
	})

	t.Run("RestoresFromTheCacheOnAFreshExecutor", func(t *testing.T) {
		skip.If(t, !sdk.InCircleCIJob(), "the cache round trip needs a CircleCI job's cache")
		assert.NilError(t, os.RemoveAll(s.InstallDir))
		out := run(t, "", installArgs...)
		assert.Assert(t, strings.Contains(out, "restored Ruby "+version+" from cache"),
			"expected the install saved above to be restored; got: %s", out)
		got, err := sdktest.LaterStep(t, "ruby", "-v")
		assert.NilError(t, err, got)
		assert.Assert(t, strings.HasPrefix(strings.TrimSpace(got), rubyV), "ruby -v: %s", got)
	})

	if strings.Contains(out, "with rbenv") {
		return
	}
	t.Run("NotIntoADirectoryOwnedByAnotherTool", func(t *testing.T) {
		for _, part := range strings.Split(filepath.ToSlash(linked), "/") {
			assert.Assert(t, part != "shims",
				"a version manager rewrites its shims directory, taking the link with it: %s", linked)
		}
	})
}

// isWithin reports whether path is inside dir, as the filesystem compares
// them: through symlinks, such as macOS's /var to /private/var, and on
// Windows ignoring case and which way the slashes go.
func isWithin(path, dir string) bool {
	path, dir = realPath(filepath.FromSlash(path)), realPath(dir)
	if runtime.GOOS == "windows" {
		path, dir = strings.ToLower(path), strings.ToLower(dir)
	}
	return strings.HasPrefix(path, dir+string(filepath.Separator))
}

func realPath(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return filepath.Clean(p)
}
