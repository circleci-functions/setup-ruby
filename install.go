package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/circleci-functions/toolkit/pathlink"
	"github.com/circleci-functions/toolkit/sdk"

	"github.com/circleci-functions/setup-ruby/rubydist"
	"github.com/circleci-functions/setup-ruby/rubypath"
	"github.com/circleci-functions/setup-ruby/rubyversion"
)

// live is where to look for a release newer than the built-in catalog. It's a
// variable so tests can point it at a fake server.
var live = rubydist.DefaultLive

// installCmd installs a relocatable Ruby build for the running platform and
// makes ruby, gem and bundle resolvable by name for the rest of the job.
type installCmd struct {
	RubyVersion      string `help:"Ruby version to install: an exact version (3.4.1), a major or major.minor prefix (3, 3.4) or latest. Defaults to the version in .ruby-version, .tool-versions or mise.toml in --working-directory, then the Ruby already on PATH, then the newest stable release." example:"3.4"`
	RubyVersionFile  string `help:"File to read the Ruby version from: .ruby-version, .tool-versions or mise.toml." type:"path"`
	WorkingDirectory string `help:"Directory to look for version files and Gemfile.lock in." default:"."`
	Rubygems         string `help:"RubyGems version: default keeps the one Ruby ships, latest runs gem update --system, and a version such as 3.6.9 updates to it when it is newer than the one Ruby ships." default:"default"`
	Bundler          string `help:"Bundler version to install: Gemfile.lock installs the one the lock file in --working-directory was written with, default keeps the one Ruby ships, latest installs the newest, none installs none, and a version such as 2.6 or 2.6.9 installs the newest matching release." default:"Gemfile.lock"`
	WindowsToolchain string `help:"On Windows, default installs the MSYS2 build tools RubyInstaller compiles native extensions with, and none skips them." default:"default" enum:"default,none"`
	Cache            bool   `help:"Restore the Ruby install from the job cache, and save it after a fresh download. Set to false to disable caching." default:"true"`
	InstallDir       string `help:"Directory to install Ruby versions under." default:"~/.cache/setup-ruby" hidden:""`
	BinDir           string `help:"Link ruby, gem and bundle into this directory instead of discovering one on PATH." type:"path"`
	CacheNonce       string `help:"Partition the Ruby install cache, so a test run cannot restore an earlier run's install." hidden:""`
	DownloadsURL     string `help:"Read the list of Ruby builds from this URL instead of setup-ruby's built-in one." hidden:""`

	catalog *rubydist.Catalog
}

func (c *installCmd) Run(ctx context.Context, client *sdk.Client) error {
	wd, err := filepath.Abs(c.WorkingDirectory)
	if err != nil {
		return fmt.Errorf("resolve working-directory: %w", err)
	}
	spec, ok, err := c.spec(wd)
	if err != nil {
		return err
	}
	if !ok {
		return c.setUp(ctx, systemRuby{}, wd)
	}

	if err := rubypath.CheckLink(rubypath.Options{BinDir: c.BinDir}); err != nil {
		return fmt.Errorf("%w; put ~/.local/bin on PATH ahead of that directory, pass --bin-dir with a directory on PATH to link into, or allow passwordless sudo", err)
	}

	platform, err := rubydist.Host()
	if err != nil {
		return err
	}
	version, root, err := c.ensureInstalled(ctx, client, spec, platform)
	if err != nil {
		return err
	}
	if err := c.ensureToolchain(ctx, client, platform, root); err != nil {
		return err
	}
	if err := rubypath.RemoveGemHook(root, version); err != nil {
		return err
	}
	if err := c.setUp(ctx, installedRuby{root: root}, wd); err != nil {
		return err
	}

	linked, err := rubypath.Link(rubypath.Options{
		Root:    root,
		Version: version,
		BinDir:  c.BinDir,
		Dir:     wd,
		Aliases: []string{strings.TrimSpace(spec.String())},
	})
	if err != nil {
		return err
	}
	c.reportLinked(version, root, linked)
	if linked.Rbenv != "" {
		fmt.Fprintf(os.Stderr, "setup-ruby: rbenv rehashes after gem install, so the executables of what gem installs resolve by name\n")
		return nil
	}
	return reportGemExecutables(root, version, linked.Dir)
}

// setUp applies the settings that hold for whichever Ruby the job uses, for
// the project in wd.
func (c *installCmd) setUp(ctx context.Context, r rubyRunner, wd string) error {
	if err := writeGemrc(); err != nil {
		return err
	}
	if err := updateRubygems(ctx, r, c.Rubygems); err != nil {
		return err
	}
	return installBundler(ctx, r, c.Bundler, wd)
}

// reportLinked says where Link made the install at root resolvable from.
func (c *installCmd) reportLinked(version, root string, linked rubypath.Result) {
	fmt.Fprintf(os.Stderr, "setup-ruby: Ruby %s installed at %s, linked into %s\n", version, root, linked.Dir)
	if linked.Rbenv != "" {
		fmt.Fprintf(os.Stderr, "setup-ruby: registered Ruby %s with rbenv as %s, and made it rbenv's global version\n", version, linked.Rbenv)
	}
	if c.BinDir != "" {
		return
	}
	for _, name := range []string{"gem", "bundle"} {
		if other := rubypath.Shadowed(linked.Dir, name); other != "" {
			fmt.Fprintf(os.Stderr, "setup-ruby: warning: %s resolves from %s, ahead of %s; run it as ruby -S %s for the Ruby installed here\n", name, other, linked.Dir, name)
		}
	}
}

// reportGemExecutables has gem install put executables beside ruby on PATH
// where it can, and says where they go either way.
func reportGemExecutables(root, version, linkedDir string) error {
	dir, onPath, err := rubypath.GemExecutables(root, version, linkedDir)
	if err != nil {
		return err
	}
	if !onPath {
		fmt.Fprintf(os.Stderr, "setup-ruby: warning: executables from gem install land in %s, which is not on PATH; run them by their path there, or with bundle exec\n", dir)
		return nil
	}
	fmt.Fprintf(os.Stderr, "setup-ruby: executables from gem install land in %s, on PATH\n", dir)
	return nil
}

// spec returns the version spec to install, or false when nothing asks for a
// version and the Ruby already on PATH is to be used instead.
func (c *installCmd) spec(wd string) (rubyversion.Spec, bool, error) {
	raw, err := c.versionSpec(wd)
	if err != nil {
		return rubyversion.Spec{}, false, err
	}
	if raw == "" {
		if useSystemRuby() {
			return rubyversion.Spec{}, false, nil
		}
		raw = rubyversion.Latest
	}
	spec, err := rubyversion.Parse(raw)
	return spec, err == nil, err
}

// versionSpec picks the version spec from --ruby-version, --ruby-version-file,
// or the version files in wd, in that order. It returns the empty string when
// there is none of them.
func (c *installCmd) versionSpec(wd string) (string, error) {
	switch {
	case c.RubyVersion != "":
		if c.RubyVersionFile != "" {
			fmt.Fprintf(os.Stderr, "setup-ruby: both --ruby-version and --ruby-version-file are set; using --ruby-version\n")
		}
		return c.RubyVersion, nil
	case c.RubyVersionFile != "":
		spec, err := rubyversion.FromFile(c.RubyVersionFile)
		if err != nil {
			return "", fmt.Errorf("ruby-version-file: %w", err)
		}
		fmt.Fprintf(os.Stderr, "setup-ruby: read Ruby version %q from %s\n", spec, c.RubyVersionFile)
		return spec, nil
	}
	spec, source, err := rubyversion.Default(wd)
	if err != nil {
		return "", err
	}
	if source != "" {
		fmt.Fprintf(os.Stderr, "setup-ruby: read Ruby version %q from %s\n", spec, source)
	}
	return spec, nil
}

// useSystemRuby reports whether a Ruby is already on PATH, which is what to
// use when nothing asks for a version.
func useSystemRuby() bool {
	files := strings.Join(rubyversion.DefaultFiles, ", ")
	bin, err := exec.LookPath("ruby")
	if err == nil {
		out, rerr := pathlink.Command(bin, "-v").Output()
		if v := strings.TrimSpace(string(out)); rerr == nil && strings.HasPrefix(v, "ruby ") {
			fmt.Fprintf(os.Stderr, "setup-ruby: no --ruby-version, --ruby-version-file, or %s; using %s at %s\n", files, v, bin)
			return true
		}
	}
	fmt.Fprintf(os.Stderr, "setup-ruby: no --ruby-version, --ruby-version-file, or %s, and no Ruby on PATH; installing the newest stable release\n", files)
	return false
}

// ensureInstalled returns the version spec resolves to and the root of its
// install, restoring it from the job cache or downloading it unless an
// earlier run already did.
func (c *installCmd) ensureInstalled(ctx context.Context, client *sdk.Client, spec rubyversion.Spec, platform rubydist.Platform) (version, root string, err error) {
	installDir, err := expandHome(c.InstallDir)
	if err != nil {
		return "", "", err
	}
	build, err := c.resolve(ctx, spec, platform)
	if err != nil {
		return "", "", err
	}
	root = filepath.Join(installDir, build.Version+"-"+platform.String())
	key := c.cacheKey("setup-ruby-v1-"+build.Version+"-"+platform.String()+"-"+buildID(build), root)
	if c.restore(ctx, client, key, "Ruby "+build.Version, build, root) {
		return build.Version, root, nil
	}
	return build.Version, root, c.download(ctx, client, key, "Ruby "+build.Version, build, root)
}

// ensureToolchain puts the MSYS2 build tools in the msys64 directory of a
// Windows install at root, where RubyInstaller looks for them when a gem
// needs compiling.
func (c *installCmd) ensureToolchain(ctx context.Context, client *sdk.Client, platform rubydist.Platform, root string) error {
	if platform.OS != "windows" || c.WindowsToolchain == "none" {
		return nil
	}
	catalog, err := c.loadCatalog(ctx)
	if err != nil {
		return err
	}
	build, ok := catalog.Toolchain(platform)
	if !ok {
		return fmt.Errorf("no MSYS2 build tools for %s; pass --windows-toolchain none to skip them", platform)
	}
	dest := filepath.Join(root, "msys64")
	key := c.cacheKey("setup-ruby-msys2-v1-"+build.Version+"-"+platform.String()+"-"+buildID(build), dest)
	name := "MSYS2 build tools " + build.Version
	if c.restore(ctx, client, key, name, build, dest) {
		return nil
	}
	return c.download(ctx, client, key, name, build, dest)
}

// resolve returns the build of the newest release matching spec for platform.
// It looks in --downloads-url when that is set, and otherwise in the catalog
// built into setup-ruby, looking for an exact release newer than that where
// its builds are published.
func (c *installCmd) resolve(ctx context.Context, spec rubyversion.Spec, platform rubydist.Platform) (rubydist.Build, error) {
	catalog, err := c.loadCatalog(ctx)
	if err != nil {
		return rubydist.Build{}, err
	}
	versions := catalog.Versions(platform)
	if len(versions) == 0 {
		return rubydist.Build{}, fmt.Errorf("no Ruby builds for %s", platform)
	}
	version, err := spec.Resolve(versions)
	if err == nil {
		build, _ := catalog.Find(platform, version)
		fmt.Fprintf(os.Stderr, "setup-ruby: resolved %q to Ruby %s (%s)\n", spec, version, platform)
		return build, nil
	}
	exactVersion, exact := spec.Exact()
	if !errors.Is(err, rubyversion.ErrNoMatch) || !exact || c.DownloadsURL != "" {
		return rubydist.Build{}, fmt.Errorf("%w for %s in setup-ruby's list of builds; upgrade setup-ruby for newer releases", err, platform)
	}
	fmt.Fprintf(os.Stderr, "setup-ruby: Ruby %s is not in setup-ruby's list of builds for %s; looking where its builds are published\n", exactVersion, platform)
	build, lerr := live.Find(ctx, platform, exactVersion)
	if lerr != nil {
		return rubydist.Build{}, fmt.Errorf("%w for %s: %w", err, platform, lerr)
	}
	return build, nil
}

func (c *installCmd) loadCatalog(ctx context.Context) (rubydist.Catalog, error) {
	if c.catalog != nil {
		return *c.catalog, nil
	}
	var catalog rubydist.Catalog
	var err error
	if c.DownloadsURL != "" {
		catalog, err = rubydist.Fetch(ctx, c.DownloadsURL)
	} else {
		catalog, err = rubydist.Snapshot()
	}
	if err != nil {
		return rubydist.Catalog{}, err
	}
	c.catalog = &catalog
	return catalog, nil
}

// restore reports whether root holds build from an earlier run or, failing
// that, the job cache. Whatever else is at root is removed first, so that a
// restore cannot mix with it.
func (c *installCmd) restore(ctx context.Context, client *sdk.Client, key, name string, build rubydist.Build, root string) bool {
	if rubydist.Installed(root, build) {
		return true
	}
	if !c.Cache {
		return false
	}
	if err := os.RemoveAll(root); err != nil {
		return false
	}
	res, err := client.RestoreCache(ctx, []string{key}, sdk.WithStepName("Restoring "+name))
	sdk.LogCacheErr("restore", err)
	if err == nil && res.Raw != "success" {
		fmt.Fprintf(os.Stderr, "setup-ruby: restoring %s from cache did not succeed (%s); downloading it instead\n", name, res.Raw)
	}
	if err != nil || res.Raw != "success" || !rubydist.Installed(root, build) {
		return false
	}
	fmt.Fprintf(os.Stderr, "setup-ruby: restored %s from cache\n", name)
	return true
}

// download installs build at root, and saves it to the job cache.
func (c *installCmd) download(ctx context.Context, client *sdk.Client, key, name string, build rubydist.Build, root string) error {
	if err := rubydist.Install(ctx, build, root); err != nil {
		return err
	}
	if c.Cache {
		sdk.LogCacheErr("save", client.SaveCache(ctx, key, []string{root}, sdk.WithStepName("Saving "+name)))
	}
	return nil
}

// cacheKey returns the key what is installed at root is cached under. The
// cache restores to the absolute path it saved, so the key carries a hash of
// root: executors whose $HOME differs must not share an entry that restores
// somewhere only one of them looks.
func (c *installCmd) cacheKey(prefix, root string) string {
	sum := sha256.Sum256([]byte(root))
	key := fmt.Sprintf("%s-%x", prefix, sum[:4])
	if c.CacheNonce != "" {
		key += "-" + c.CacheNonce
	}
	return key
}

// buildID shortens the SHA-256 of b for a cache key, which has to change when
// a version is rebuilt: a key, once saved, cannot be saved over.
func buildID(b rubydist.Build) string {
	sum := strings.ToLower(b.SHA256)
	if len(sum) > 8 {
		return sum[:8]
	}
	return sum
}

// writeGemrc stops gem install generating documentation, which a CI job
// never reads, unless the user has a ~/.gemrc of their own.
func writeGemrc() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	path := filepath.Join(home, ".gemrc")
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	if err := os.WriteFile(path, []byte("gem: --no-document\n"), 0644); err != nil { //nolint:gosec // gem config, not secret
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// expandHome resolves a leading ~ against $HOME. kong's path type resolves it
// against the passwd entry instead, which ignores a redirected $HOME.
func expandHome(path string) (string, error) {
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return filepath.Abs(path)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, strings.TrimPrefix(path, "~")), nil
}
