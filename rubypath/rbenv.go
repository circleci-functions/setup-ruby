package rubypath

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/circleci-functions/toolkit/pathlink"
)

// rbenvRoot returns the root of the rbenv whose shims ruby resolves through
// on path, if it does.
func rbenvRoot(path string) (string, bool) {
	dirs := pathlink.SearchDirs(path)
	i := pathlink.IndexOf(dirs, "ruby")
	if i < 0 || filepath.Base(dirs[i]) != "shims" {
		return "", false
	}
	root := filepath.Dir(dirs[i])
	if info, err := os.Stat(filepath.Join(root, "versions")); err != nil || !info.IsDir() {
		return "", false
	}
	return root, true
}

// linkWithRbenv registers the install with the rbenv at root, makes it
// rbenv's global version, and rehashes, so rbenv's shims run it. It also has
// gem install rehash after installing a gem with executables, so they get
// shims too: not every rbenv does that by itself.
func linkWithRbenv(root string, opts Options) (Result, error) {
	bin, rbenv, err := rbenvCommand(root, opts.Path, opts.Dir)
	if err != nil {
		return Result{}, err
	}
	name, err := registerWithRbenv(root, opts.Root, opts.Version, opts.Aliases)
	if err != nil {
		return Result{}, err
	}
	for _, args := range [][]string{{"global", name}, {"rehash"}} {
		if _, err := rbenv(args...); err != nil {
			return Result{}, err
		}
	}
	rehash := "Gem.post_install do |installer|\n" +
		"  system({ \"RBENV_ROOT\" => " + rubyString(root) + " }, " + rubyString(bin) + ", \"rehash\") unless installer.spec.executables.empty?\n" +
		"end\n"
	if err := writeGemHook(opts.Root, opts.Version, rehash); err != nil {
		return Result{}, err
	}
	shims := filepath.Join(root, "shims")
	if err := reportsVersion(filepath.Join(shims, "ruby"), opts.Dir, opts.Version); err != nil {
		selected, _ := rbenv("version")
		return Result{}, fmt.Errorf("rbenv runs %s here, whose ruby %w: a .ruby-version file or $RBENV_VERSION takes precedence over rbenv's global version",
			strings.TrimSpace(selected), err)
	}
	return Result{Dir: shims, Rbenv: name}, nil
}

// registerWithRbenv links the install at install into the versions of the
// rbenv at root, named for its version, and returns the name used. A version
// rbenv installed itself under that name is left alone, and the install
// registered beside it instead. The install is also linked under each of
// aliases rbenv has no version of its own by: rbenv looks a .ruby-version up
// by name only, so one asking for 3.4 finds nothing under 3.4.1.
func registerWithRbenv(root, install, version string, aliases []string) (string, error) {
	name := ""
	for _, candidate := range []string{version, version + "-setup-ruby"} {
		linked, err := linkVersion(root, install, candidate)
		if err != nil {
			return "", err
		}
		if linked {
			name = candidate
			break
		}
	}
	if name == "" {
		return "", fmt.Errorf("register with rbenv: %s already holds %s and %s-setup-ruby", filepath.Join(root, "versions"), version, version)
	}
	for _, alias := range aliases {
		if strings.TrimPrefix(alias, "ruby-") == version || !filepath.IsLocal(alias) || filepath.Base(alias) != alias {
			continue
		}
		if _, err := linkVersion(root, install, alias); err != nil {
			return "", err
		}
	}
	return name, nil
}

// linkVersion links the install at install into the versions of the rbenv at
// root as name, replacing a link an earlier run left there. It reports false,
// linking nothing, where rbenv installed a version of its own under name.
func linkVersion(root, install, name string) (bool, error) {
	target := filepath.Join(root, "versions", name)
	info, err := os.Lstat(target)
	if err == nil && info.Mode()&os.ModeSymlink == 0 {
		return false, nil
	}
	if err == nil {
		if err := os.Remove(target); err != nil {
			return false, fmt.Errorf("register with rbenv: %w", err)
		}
	}
	if err := os.Symlink(install, target); err != nil {
		return false, fmt.Errorf("register with rbenv: %w", err)
	}
	return true, nil
}

// rbenvCommand returns the rbenv at root, the rbenv on path or else the one
// inside root, and a function running it in dir.
func rbenvCommand(root, path, dir string) (string, func(...string) (string, error), error) {
	bin := ""
	dirs := pathlink.SearchDirs(path)
	if i := pathlink.IndexOf(dirs, "rbenv"); i >= 0 {
		bin = filepath.Join(dirs[i], "rbenv")
	}
	for _, candidate := range []string{filepath.Join(root, "libexec", "rbenv"), filepath.Join(root, "bin", "rbenv")} {
		if bin != "" {
			break
		}
		if _, err := os.Stat(candidate); err == nil {
			bin = candidate
		}
	}
	if bin == "" {
		return "", nil, fmt.Errorf("ruby resolves through rbenv's shims in %s, but there is no rbenv to register the install with", root)
	}
	return bin, func(args ...string) (string, error) {
		cmd := exec.Command(bin, args...) //nolint:gosec // the rbenv on PATH or in its root
		cmd.Env = append(os.Environ(), "RBENV_ROOT="+root)
		cmd.Dir = dir
		out, err := cmd.Output()
		if err != nil {
			var stderr string
			if exitErr := (*exec.ExitError)(nil); errors.As(err, &exitErr) {
				stderr = strings.TrimSpace(string(exitErr.Stderr))
			}
			return string(out), fmt.Errorf("rbenv %s: %w: %s", strings.Join(args, " "), err, stderr)
		}
		return string(out), nil
	}, nil
}
