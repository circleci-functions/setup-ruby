package rubypath

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/skip"

	"github.com/circleci-functions/setup-ruby/internal/testing/fakerbenv"
)

// fakeInstall writes a Unix Ruby install whose ruby reports 3.4.1.
func fakeInstall(t *testing.T) string {
	t.Helper()
	skip.If(t, runtime.GOOS == "windows", "the fake install's ruby is a shell script")
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	assert.NilError(t, os.MkdirAll(bin, 0750))
	assert.NilError(t, os.WriteFile(filepath.Join(bin, "ruby"), []byte("#!/bin/sh\necho 'ruby 3.4.1 (2024-12-25 revision 48d4efcb85) +PRISM [fake]'\n"), 0700)) //nolint:gosec // must be executable
	for _, name := range []string{"gem", "bundle", "irb"} {
		assert.NilError(t, os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\n"), 0700)) //nolint:gosec // must be executable
	}
	return root
}

func TestLink_DiscoversDirOnPath(t *testing.T) {
	root := fakeInstall(t)
	onPath := t.TempDir()

	res, err := Link(Options{Root: root, Version: "3.4.1", Path: onPath})
	assert.NilError(t, err)
	assert.DeepEqual(t, res, Result{Dir: onPath})
	for _, name := range []string{"ruby", "gem", "bundle", "irb"} {
		target, err := os.Readlink(filepath.Join(onPath, name))
		assert.NilError(t, err, name)
		assert.Equal(t, target, filepath.Join(root, "bin", name))
	}
	_, err = os.Lstat(filepath.Join(onPath, "rake"))
	assert.Assert(t, os.IsNotExist(err), "a script the install lacks should not be linked")
}

func TestLink_BinDir(t *testing.T) {
	root := fakeInstall(t)
	binDir := t.TempDir()

	res, err := Link(Options{Root: root, Version: "3.4.1", BinDir: binDir})
	assert.NilError(t, err)
	assert.Equal(t, res.Dir, binDir)
	_, err = os.Stat(filepath.Join(binDir, "gem"))
	assert.NilError(t, err)
}

func TestVerify(t *testing.T) {
	root := fakeInstall(t)
	other := fakeInstall(t)
	broken := fakeInstall(t)
	assert.NilError(t, os.WriteFile(filepath.Join(broken, "bin", "ruby"), []byte("#!/bin/sh\necho 'GLIBC_2.17 not found' >&2\nexit 1\n"), 0700)) //nolint:gosec // must be executable

	link := filepath.Join(t.TempDir(), "ruby")
	assert.NilError(t, os.Symlink(filepath.Join(root, "bin", "ruby"), link))

	tests := []struct {
		name    string
		from    string
		version string
		bin     string
		wantErr string
	}{
		{name: "this install", from: root, version: "3.4.1", bin: link},
		{name: "another version", from: root, version: "3.3.6", bin: link, wantErr: `rather than Ruby 3.3.6`},
		{name: "a lookalike", from: other, version: "3.4.1", bin: link, wantErr: "is not " + filepath.Join(other, "bin", "ruby")},
		{name: "fails to run", from: broken, version: "3.4.1", bin: filepath.Join(broken, "bin", "ruby"), wantErr: "GLIBC_2.17 not found"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := verify(filepath.Join(tc.from, "bin"), tc.version)(tc.bin)
			if tc.wantErr == "" {
				assert.NilError(t, err)
				return
			}
			assert.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestReportsVersion_Prerelease(t *testing.T) {
	skip.If(t, runtime.GOOS == "windows", "the fake ruby is a shell script")
	bin := filepath.Join(t.TempDir(), "ruby")
	assert.NilError(t, os.WriteFile(bin, []byte("#!/bin/sh\necho 'ruby 3.5.0preview1 (2025-04-18 master 0000000000) +PRISM [fake]'\n"), 0700)) //nolint:gosec // must be executable

	assert.NilError(t, reportsVersion(bin, "", "3.5.0-preview1"))
}

func TestLink_RegistersWithRbenv(t *testing.T) {
	install := fakeInstall(t)
	root := fakerbenv.New(t, "3.3.0")

	res, err := Link(Options{Root: install, Version: "3.4.1", Path: filepath.Join(root, "shims")})
	assert.NilError(t, err)
	assert.DeepEqual(t, res, Result{Dir: filepath.Join(root, "shims"), Rbenv: "3.4.1"})

	target, err := os.Readlink(filepath.Join(root, "versions", "3.4.1"))
	assert.NilError(t, err)
	assert.Equal(t, target, install)
	global, err := os.ReadFile(filepath.Join(root, "version"))
	assert.NilError(t, err)
	assert.Equal(t, string(global), "3.4.1\n")
	_, err = os.Stat(filepath.Join(root, "rehashed"))
	assert.NilError(t, err, "expected rbenv to rehash")
}

func TestLink_RegistersWithRbenvBesideItsOwnInstall(t *testing.T) {
	install := fakeInstall(t)
	root := fakerbenv.New(t, "3.4.1")

	res, err := Link(Options{Root: install, Version: "3.4.1", Path: filepath.Join(root, "shims")})
	assert.NilError(t, err)
	assert.Equal(t, res.Rbenv, "3.4.1-setup-ruby")
	_, err = os.Stat(filepath.Join(root, "versions", "3.4.1", "bin", "ruby"))
	assert.NilError(t, err, "rbenv's own install was disturbed")
}

func TestLink_RerunReplacesItsRbenvLink(t *testing.T) {
	root := fakerbenv.New(t, "3.3.0")
	for range 2 {
		install := fakeInstall(t)
		res, err := Link(Options{Root: install, Version: "3.4.1", Path: filepath.Join(root, "shims")})
		assert.NilError(t, err)
		assert.Equal(t, res.Rbenv, "3.4.1")
		target, err := os.Readlink(filepath.Join(root, "versions", "3.4.1"))
		assert.NilError(t, err)
		assert.Equal(t, target, install)
	}
}

func TestLink_RbenvVersionOverridesGlobal(t *testing.T) {
	install := fakeInstall(t)
	root := fakerbenv.New(t, "3.3.0")
	t.Setenv("RBENV_VERSION", "3.3.0")

	_, err := Link(Options{Root: install, Version: "3.4.1", Path: filepath.Join(root, "shims")})
	assert.ErrorContains(t, err, "rbenv runs 3.3.0")
	assert.ErrorContains(t, err, "$RBENV_VERSION takes precedence")
}

func TestLink_RbenvVersionFileInDir(t *testing.T) {
	install := fakeInstall(t)
	root := fakerbenv.New(t, "3.3.0")
	dir := t.TempDir()
	assert.NilError(t, os.WriteFile(filepath.Join(dir, ".ruby-version"), []byte("3.3.0\n"), 0600))

	_, err := Link(Options{Root: install, Version: "3.4.1", Path: filepath.Join(root, "shims"), Dir: dir})
	assert.ErrorContains(t, err, "rbenv runs 3.3.0")
}

func TestLink_RbenvAliases(t *testing.T) {
	install := fakeInstall(t)
	root := fakerbenv.New(t, "3.4")
	dir := t.TempDir()
	assert.NilError(t, os.WriteFile(filepath.Join(dir, ".ruby-version"), []byte("ruby-3\n"), 0600))

	res, err := Link(Options{Root: install, Version: "3.4.1", Path: filepath.Join(root, "shims"), Dir: dir, Aliases: []string{"ruby-3", "3.4", "ruby-3.4.1", "../3"}})
	assert.NilError(t, err)
	assert.Equal(t, res.Rbenv, "3.4.1")
	target, err := os.Readlink(filepath.Join(root, "versions", "ruby-3"))
	assert.NilError(t, err)
	assert.Equal(t, target, install)
	_, err = os.Stat(filepath.Join(root, "versions", "3.4", "bin", "ruby"))
	assert.NilError(t, err, "rbenv's own install was disturbed")
	for _, name := range []string{"ruby-3.4.1", filepath.Join("..", "3")} {
		_, err = os.Lstat(filepath.Join(root, "versions", name))
		assert.Assert(t, os.IsNotExist(err), "%s should not be registered", name)
	}
}

func TestGemExecutables(t *testing.T) {
	skip.If(t, runtime.GOOS == "windows", "Windows builds keep gem executables in the install")
	t.Setenv("GEM_HOME", "")
	root := fakeInstall(t)
	linked := t.TempDir()
	hook := filepath.Join(root, "lib", "ruby", "site_ruby", "3.4.0", "rubygems", "defaults", "operating_system.rb")

	dir, onPath, err := GemExecutables(root, "3.4.1", linked)
	assert.NilError(t, err)
	assert.Equal(t, dir, linked)
	assert.Assert(t, onPath)
	data, err := os.ReadFile(hook)
	assert.NilError(t, err)
	assert.Assert(t, strings.Contains(string(data), "def self.default_bindir\n    '"+linked+"'"), "hook: %s", data)
	assert.Assert(t, strings.Contains(string(data), loadShippedHook), "hook should load the install's own first: %s", data)

	t.Setenv("GEM_HOME", filepath.Join(t.TempDir(), "gems"))
	dir, onPath, err = GemExecutables(root, "3.4.1", linked)
	assert.NilError(t, err)
	assert.Equal(t, dir, filepath.Join(os.Getenv("GEM_HOME"), "bin"))
	assert.Assert(t, !onPath)
	_, err = os.Stat(hook)
	assert.Assert(t, os.IsNotExist(err), "the hook should be removed where $GEM_HOME decides")
}

func TestRemoveGemHook_LeavesAnotherHook(t *testing.T) {
	root := t.TempDir()
	hook := gemHook(root, "3.4.1")
	assert.NilError(t, os.MkdirAll(filepath.Dir(hook), 0750))
	assert.NilError(t, os.WriteFile(hook, []byte("# the install's own\n"), 0600))

	assert.NilError(t, RemoveGemHook(root, "3.4.1"))
	_, err := os.Stat(hook)
	assert.NilError(t, err, "a hook setup-ruby did not write must be left alone")
}

func TestGemHook_Prerelease(t *testing.T) {
	assert.Equal(t, gemHook("/r", "3.5.0-preview1"), filepath.Join("/r", "lib", "ruby", "site_ruby", "3.5.0", "rubygems", "defaults", "operating_system.rb"))
}

func TestLink_RbenvRehashesAfterGemInstall(t *testing.T) {
	install := fakeInstall(t)
	root := fakerbenv.New(t, "3.3.0")

	_, err := Link(Options{Root: install, Version: "3.4.1", Path: filepath.Join(root, "shims")})
	assert.NilError(t, err)
	data, err := os.ReadFile(gemHook(install, "3.4.1"))
	assert.NilError(t, err)
	want := `system({ "RBENV_ROOT" => '` + root + `' }, '` + filepath.Join(root, "bin", "rbenv") + `', "rehash")`
	assert.Assert(t, strings.Contains(string(data), want), "hook: %s", data)
}

// TestGemHooks_AreValidRuby checks every hook setup-ruby writes parses, with
// whatever Ruby is on PATH.
func TestGemHooks_AreValidRuby(t *testing.T) {
	ruby, err := exec.LookPath("ruby")
	skip.If(t, err != nil, "no ruby on PATH to check the hooks with")
	for name, body := range map[string]string{
		"bin dir":       "module Gem\n  def self.default_bindir\n    " + rubyString(`C:\it's`) + "\n  end\nend\n",
		"windows stubs": absoluteWindowsStubs,
		"rbenv":         "Gem.post_install do |installer|\n  system({ \"RBENV_ROOT\" => 'r' }, 'b', \"rehash\") unless installer.spec.executables.empty?\nend\n",
	} {
		t.Run(name, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "operating_system.rb")
			assert.NilError(t, os.WriteFile(file, []byte(hookMarker+".\n"+loadShippedHook+body), 0600))
			out, err := exec.Command(ruby, "-c", file).CombinedOutput()
			assert.NilError(t, err, "%s", out)
		})
	}
}
