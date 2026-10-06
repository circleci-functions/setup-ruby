// Package rubypath makes an unpacked Ruby install usable by name for the rest
// of a CircleCI job.
package rubypath

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/circleci-functions/toolkit/pathlink"
)

// Scripts are the executables besides ruby that a Ruby install ships and
// Link makes resolvable, where the install has them.
var Scripts = []string{"gem", "bundle", "bundler", "irb", "rake", "erb", "rdoc", "ri"}

// Options configures Link.
type Options struct {
	// Root is the unpacked install to make resolvable.
	Root string
	// Version is the version ruby -v must report, e.g. 3.4.1.
	Version string
	// BinDir links into exactly this directory, skipping PATH discovery.
	BinDir string
	// Path is the search path to use. Empty means this process's PATH.
	Path string
}

// BinDir returns the directory of an install at root holding its executables.
func BinDir(root string) string {
	return filepath.Join(root, "bin")
}

// Executable returns the ruby executable of the install at root.
func Executable(root string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(BinDir(root), "ruby.exe")
	}
	return filepath.Join(BinDir(root), "ruby")
}

// Installed reports whether root holds a Ruby install.
func Installed(root string) bool {
	_, err := os.Stat(Executable(root))
	return err == nil
}

// Result describes where Link made ruby resolvable from.
type Result struct {
	// Dir is the directory ruby resolves from.
	Dir string
	// Rbenv is the name the install was registered with rbenv as, when ruby
	// was already on PATH through rbenv's shims, and "" otherwise.
	Rbenv string
}

// Link makes ruby, and those of Scripts the install has, resolvable by name
// from a directory already on PATH.
//
// Where ruby is already on PATH through rbenv's shims, it registers the
// install with rbenv and makes it rbenv's global version instead, since rbenv
// rewrites its shims directory and would take any link there with it.
func Link(opts Options) (Result, error) {
	if opts.BinDir == "" {
		if root, ok := rbenvRoot(opts.Path); ok {
			return linkWithRbenv(root, opts)
		}
	}

	from := BinDir(opts.Root)
	lopts := pathlink.Options{
		Names:     []string{"ruby"},
		FromDir:   from,
		Preferred: pathlink.UserBinDirs(),
		Dir:       opts.BinDir,
		Path:      opts.Path,
		Verify:    verify(from, opts.Version),
	}
	if runtime.GOOS != "windows" {
		lopts.Names = append(lopts.Names, present(from, Scripts)...)
	}
	res, err := pathlink.Ensure(lopts)
	if err != nil {
		return Result{}, err
	}
	if runtime.GOOS == "windows" {
		if err := writeScriptShims(res.Dir, opts.Root); err != nil {
			return Result{}, err
		}
	}
	return Result{Dir: res.Dir}, nil
}

// writeScriptShims makes the scripts of a Windows install resolvable from
// dir, in cmd, PowerShell and Git Bash. RubyInstaller ships each as a Ruby
// script beside a .bat or .cmd wrapper, which pathlink, linking .exe files,
// cannot point at, so the shims run the script with ruby.exe directly.
func writeScriptShims(dir, root string) error {
	ruby := Executable(root)
	for _, name := range present(BinDir(root), Scripts) {
		script := filepath.Join(BinDir(root), name)
		shims := map[string]string{
			name + ".bat": fmt.Sprintf("@echo off\r\n\"%s\" \"%s\" %%*\r\n", ruby, script),
			name:          fmt.Sprintf("#!/bin/sh\nexec \"%s\" \"%s\" \"$@\"\n", filepath.ToSlash(ruby), filepath.ToSlash(script)),
		}
		for file, content := range shims {
			if err := os.WriteFile(filepath.Join(dir, file), []byte(content), 0755); err != nil { //nolint:gosec // must be executable
				return err
			}
		}
	}
	return nil
}

// Shadowed returns the directory name resolves from on PATH when that is not
// dir, or "" when it is dir or resolves nowhere.
func Shadowed(dir, name string) string {
	dirs := pathlink.SearchDirs("")
	i := pathlink.IndexOf(dirs, name)
	if i < 0 || sameFile(dirs[i], dir) {
		return ""
	}
	return dirs[i]
}

// verify accepts only a ruby reporting version and, on Unix, resolving to
// this install's own, so a preinstalled ruby that happens to answer the same
// does not pass for it. On Windows the links are shims that call ruby.exe by
// absolute path, so there only the version can identify them.
func verify(from, version string) func(string) error {
	want := filepath.Join(from, "ruby")
	return func(bin string) error {
		if err := reportsVersion(bin, version); err != nil {
			return err
		}
		if runtime.GOOS != "windows" && !sameFile(bin, want) {
			return fmt.Errorf("is not %s", want)
		}
		return nil
	}
}

// reportsVersion checks the ruby at bin runs and reports version. ruby -v
// writes a prerelease without the hyphen in its name: 3.5.0preview1.
func reportsVersion(bin, version string) error {
	out, err := pathlink.Command(bin, "-v").CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to run: %w: %s", err, strings.TrimSpace(string(out)))
	}
	got := strings.TrimSpace(string(out))
	if !strings.HasPrefix(got, "ruby "+strings.ReplaceAll(version, "-", "")+" ") {
		return fmt.Errorf("reports %q rather than Ruby %s", got, version)
	}
	return nil
}

// present returns the names that exist in dir.
func present(dir string, names []string) []string {
	var out []string
	for _, name := range names {
		if _, err := os.Lstat(filepath.Join(dir, name)); err == nil {
			out = append(out, name)
		}
	}
	return out
}

func sameFile(a, b string) bool {
	ra, errA := filepath.EvalSymlinks(a)
	rb, errB := filepath.EvalSymlinks(b)
	if errA != nil || errB != nil {
		return filepath.Clean(a) == filepath.Clean(b)
	}
	if runtime.GOOS == "windows" {
		return strings.EqualFold(ra, rb)
	}
	return ra == rb
}
