package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/circleci-functions/setup-ruby/ruby"
)

// installBundler installs the Bundler want asks for into r:
//
//   - Gemfile.lock: the version the project's lock file in wd was written
//     with, or else as default
//   - default: none, leaving the Bundler Ruby ships
//   - latest: the newest Bundler r can run
//   - none: none
//   - a version such as 2, 2.6 or 2.6.9: the newest matching release
func installBundler(ctx context.Context, r rubyRunner, want, wd string) error {
	if want == "Gemfile.lock" {
		want = "default"
		if lock := lockfile(wd); lock != "" {
			if data, err := os.ReadFile(lock); err == nil { //nolint:gosec // the project's own lock file
				if v := ruby.LockedBundler(string(data)); v != "" {
					fmt.Fprintf(os.Stderr, "setup-ruby: %s was written with Bundler %s\n", lock, v)
					want = v
				}
			}
		}
	}
	switch want {
	case "none":
		return nil
	case "default":
		fmt.Fprintln(os.Stderr, "setup-ruby: using the Bundler Ruby ships")
		return nil
	case "latest":
		fmt.Fprintln(os.Stderr, "setup-ruby: installing the latest Bundler")
		return runGem(ctx, r, "install", "bundler")
	}
	constraint := want
	if strings.Count(want, ".") < 2 {
		constraint = "~> " + want + ".0"
	}
	if r.command(ctx, "gem", "list", "--installed", "bundler", "--version", constraint).Run() == nil {
		fmt.Fprintf(os.Stderr, "setup-ruby: Bundler %s already installed\n", constraint)
		return nil
	}
	fmt.Fprintf(os.Stderr, "setup-ruby: installing Bundler %s\n", constraint)
	return runGem(ctx, r, "install", "bundler", "-v", constraint)
}

// lockfile returns the lock file of the project in wd, found as Bundler
// finds it, or "" when there is no Gemfile there.
func lockfile(wd string) string {
	resolve := func(p string) string {
		if filepath.IsAbs(p) {
			return p
		}
		return filepath.Join(wd, p)
	}
	if lock := os.Getenv("BUNDLE_LOCKFILE"); lock != "" {
		return resolve(lock)
	}
	gemfile := os.Getenv("BUNDLE_GEMFILE")
	if gemfile == "" {
		for _, name := range []string{"Gemfile", "gems.rb"} {
			if _, err := os.Stat(filepath.Join(wd, name)); err == nil {
				gemfile = name
				break
			}
		}
	}
	switch {
	case gemfile == "":
		return ""
	case filepath.Base(gemfile) == "gems.rb":
		return filepath.Join(filepath.Dir(resolve(gemfile)), "gems.locked")
	}
	return resolve(gemfile) + ".lock"
}
