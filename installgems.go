package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/circleci-functions/toolkit/sdk"

	"github.com/circleci-functions/setup-ruby/ruby"
)

// installGemsCmd installs a project's gems with Bundler, restoring the gem
// cache before the install and saving it afterwards.
type installGemsCmd struct {
	AppDir                         string `help:"Directory containing the Gemfile." default:"."`
	Gemfile                        string `help:"Gemfile name." default:"Gemfile"`
	Path                           string `help:"Gem install path; the default enables bundler deployment mode." default:"./vendor/bundle"`
	Key                            string `help:"Cache key prefix." default:"gems-v1"`
	CleanBundle                    bool   `help:"Run 'bundle clean --force' after install."`
	OverrideCacheFile              string `help:"Lock file to use for the cache key instead of the detected one."`
	Cache                          bool   `help:"Restore the gem cache before installing and save it at the end. Set to false to disable caching." default:"true"`
	IncludeArchInCacheKey          bool   `help:"Include the architecture in the cache key." default:"true"`
	IncludeBranchInCacheKey        bool   `help:"Include the branch in the cache key." default:"true"`
	OnlyUseGemfileChecksumCacheKey bool   `help:"Restore only the exact checksum key, without the loose fallback."`
}

func (c *installGemsCmd) Run(ctx context.Context, client *sdk.Client) error {
	appDir, err := filepath.Abs(c.AppDir)
	if err != nil {
		return fmt.Errorf("resolve app-dir: %w", err)
	}
	info, err := os.Stat(appDir)
	if err != nil {
		return fmt.Errorf("app-dir %q: %w", appDir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("app-dir %q is not a directory", appDir)
	}

	lockfile, hasLock := ruby.ResolveLockfile(ruby.LockfileInput{
		Dir:      appDir,
		Gemfile:  c.Gemfile,
		Override: c.OverrideCacheFile,
		DepsNext: os.Getenv("DEPENDENCIES_NEXT") != "",
	})

	var parts ruby.CacheKeyParts
	cacheable := c.Cache && hasLock
	switch {
	case cacheable:
		sum, cerr := ruby.Checksum(lockfile)
		if cerr != nil {
			return cerr
		}
		parts = ruby.CacheKeyParts{Prefix: c.Key, Checksum: sum}
		if c.IncludeArchInCacheKey {
			parts.Arch = sdk.OSArch()
		}
		if c.IncludeBranchInCacheKey {
			parts.Branch = os.Getenv("CIRCLE_BRANCH")
		}
		_, rerr := client.RestoreCache(ctx, parts.RestoreKeys(c.OnlyUseGemfileChecksumCacheKey))
		sdk.LogCacheErr("restore", rerr)
	case c.Cache:
		fmt.Fprintf(os.Stderr, "setup-ruby: no lock file found in %s, skipping cache\n", appDir)
	}

	gemfileAbs, err := filepath.Abs(filepath.Join(appDir, c.Gemfile))
	if err != nil {
		return fmt.Errorf("resolve gemfile: %w", err)
	}
	if err := configureAndInstall(ctx, appDir, gemfileAbs, c.Path, c.CleanBundle); err != nil {
		return err
	}

	if cacheable {
		save := filepath.Join(appDir, c.Path)
		sdk.LogCacheErr("save", client.SaveCache(ctx, parts.Exact(), []string{save}))
	}
	return nil
}

func configureAndInstall(ctx context.Context, dir, gemfileAbs, path string, clean bool) error {
	// The modern `bundle config set` subcommand errors when invoked with no
	// name/value; older bundler treats "set" as a config key and succeeds.
	useSet := !ruby.Succeeds(ctx, dir, "bundle", "config", "set")

	cfg := ruby.BundleConfig{
		Deployment: path == ruby.DefaultPath,
		GemfileAbs: gemfileAbs,
		Path:       path,
		OpenSSLDir: ruby.RVMOpenSSLDir(),
	}
	for _, args := range cfg.Commands(useSet) {
		if err := ruby.Run(ctx, dir, "bundle", args...); err != nil {
			return fmt.Errorf("bundle %s: %w", strings.Join(args, " "), err)
		}
	}

	if ruby.Succeeds(ctx, dir, "bundle", "check") {
		return nil
	}
	if err := ruby.Run(ctx, dir, "bundle", "install"); err != nil {
		return fmt.Errorf("bundle install: %w", err)
	}
	if clean {
		return ruby.Run(ctx, dir, "bundle", "clean", "--force")
	}
	return nil
}
