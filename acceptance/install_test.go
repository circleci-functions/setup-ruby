//go:build acceptance

package setupruby_test

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/skip"

	"github.com/circleci-functions/toolkit/sdk/sdktest"

	"github.com/circleci-functions/setup-ruby/rubydist"
	"github.com/circleci-functions/setup-ruby/rubypath"
	"github.com/circleci-functions/setup-ruby/rubyversion"
)

var exactVersion = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

// TestAcceptance_InstallsRuby installs a real Ruby into a sandbox and uses it:
// its standard library's native extensions load, it reaches rubygems.org over
// TLS, and it installs gems, both with gem and with install-gems.
func TestAcceptance_InstallsRuby(t *testing.T) {
	s := sdktest.NewSandbox(t, sdktest.WithLiveCache())
	spec := acceptanceVersion()
	installArgs := []string{
		"--ruby-version", spec,
		"--install-dir", s.InstallDir,
		"--bin-dir", s.BinDir,
		"--cache-nonce", s.CacheKeyNonce,
	}

	out := runIn(t, s, "", nil, installArgs...)
	t.Logf("setup-ruby: %s", out)
	version := reported(t, out, `Ruby (\S+) installed at`)
	if exactVersion.MatchString(spec) {
		assert.Equal(t, version, spec)
	}
	got := runTool(t, s, "", nil, s.BinDir, "ruby", "-v")
	assert.Assert(t, strings.HasPrefix(got, "ruby "+strings.ReplaceAll(version, "-", "")), "ruby -v: %s", got)

	t.Run("LoadsItsNativeExtensions", func(t *testing.T) {
		got := runTool(t, s, "", nil, s.BinDir, "ruby", "-ropenssl", "-rpsych", "-rzlib", "-rfiddle", "-e", "puts Psych.load('a: 1')['a']")
		assert.Equal(t, got, "1")
	})

	t.Run("ReachesRubygemsOverTLS", func(t *testing.T) {
		got := runTool(t, s, "", nil, s.BinDir, "ruby", "-rnet/http", "-e", `puts Net::HTTP.get_response(URI("https://rubygems.org/")).code`)
		assert.Assert(t, strings.HasPrefix(got, "2") || strings.HasPrefix(got, "3"), "GET https://rubygems.org/: %s", got)
	})

	t.Run("GemInstallsLandBesideRuby", func(t *testing.T) {
		skip.If(t, runtime.GOOS == "windows", "Windows builds keep gem executables in the install")
		runTool(t, s, "", nil, s.BinDir, "gem", "install", "dotenv", "-v", "3.1.8")
		got := runTool(t, s, "", nil, s.BinDir, "dotenv", "--version")
		assert.Assert(t, strings.Contains(got, "3.1.8"), "dotenv --version: %s", got)
	})

	t.Run("InstallsGems", func(t *testing.T) {
		app := writeApp(t)
		path := map[string]string{"PATH": s.BinDir + string(os.PathListSeparator) + os.Getenv("PATH")}
		runTool(t, s, app, path, s.BinDir, "bundle", "lock")
		out := runIn(t, s, "", path, "install-gems", "--app-dir", app, "--key", "gems-"+s.CacheKeyNonce)
		t.Logf("install-gems: %s", out)
		got := runTool(t, s, app, path, s.BinDir, "bundle", "exec", "rake", "--version")
		assert.Assert(t, strings.Contains(got, "13.2.1"), "bundle exec rake --version: %s", got)
	})

	t.Run("RestoresFromTheCache", func(t *testing.T) {
		assert.NilError(t, os.RemoveAll(s.InstallDir))
		out := runIn(t, s, "", nil, installArgs...)
		assert.Assert(t, strings.Contains(out, "restored Ruby "+version+" from cache"),
			"expected the install saved above to be restored; got: %s", out)
		got := runTool(t, s, "", nil, s.BinDir, "ruby", "-v")
		assert.Assert(t, strings.HasPrefix(got, "ruby "+strings.ReplaceAll(version, "-", "")), "ruby -v: %s", got)
	})
}

// TestAcceptance_LiveFindsWhatIsPublished checks the lookup for releases
// newer than the built-in list against what is really published, using the
// newest release the list has for this platform.
func TestAcceptance_LiveFindsWhatIsPublished(t *testing.T) {
	skip.If(t, runtime.GOOS == "windows", "RubyInstaller publishes no list of builds to look in")
	p, err := rubydist.Host()
	assert.NilError(t, err)
	c, err := rubydist.Snapshot()
	assert.NilError(t, err)
	spec, err := rubyversion.Parse("latest")
	assert.NilError(t, err)
	version, err := spec.Resolve(c.Versions(p))
	assert.NilError(t, err)

	build, err := rubydist.DefaultLive.Find(context.Background(), p, version)
	assert.NilError(t, err)
	root := filepath.Join(t.TempDir(), "ruby")
	assert.NilError(t, rubydist.Install(context.Background(), build, root))
	assert.Assert(t, rubypath.Installed(root))
}

// reported returns the first submatch of pattern in setup-ruby's output.
func reported(t *testing.T, out, pattern string) string {
	t.Helper()
	if got := find(out, pattern); got != "" {
		return got
	}
	t.Fatalf("setup-ruby did not report %q:\n%s", pattern, out)
	return ""
}

// find returns the first submatch of pattern in out, or "".
func find(out, pattern string) string {
	re := regexp.MustCompile(pattern)
	for line := range strings.SplitSeq(out, "\n") {
		if m := re.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			return m[1]
		}
	}
	return ""
}
