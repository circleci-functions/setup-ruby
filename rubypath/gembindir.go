package rubypath

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/circleci-functions/toolkit/pathlink"
)

// hookMarker starts the RubyGems hook GemExecutables writes, so a later run
// can tell its own hook from one the install shipped.
const hookMarker = "# Written by setup-ruby"

// GemExecutables sets where gem install puts the executables of the gems it
// installs into the install at root, and reports that directory and whether
// it is on PATH.
//
// Where it can, it makes that linkedDir, the directory ruby was linked into,
// so they resolve by name like ruby itself. It does that through RubyGems'
// operating_system.rb hook in the install's own site directory, which no
// other Ruby reads. Where $GEM_HOME is set it leaves gem install to put them
// in $GEM_HOME/bin. On Windows, whose builds ship a hook of their own, and
// where linkedDir needs elevation to write to, they stay in the install's bin
// directory.
func GemExecutables(root, version, linkedDir string) (dir string, onPath bool, err error) {
	if home := os.Getenv("GEM_HOME"); home != "" {
		bin := filepath.Join(home, "bin")
		return bin, onSearchPath(bin), RemoveGemHook(root, version)
	}
	if runtime.GOOS == "windows" || !pathlink.Writable(linkedDir) {
		return BinDir(root), false, RemoveGemHook(root, version)
	}
	hook := gemHook(root, version)
	if err := os.MkdirAll(filepath.Dir(hook), 0750); err != nil {
		return "", false, err
	}
	quoted := strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(linkedDir)
	content := hookMarker + ": gem install puts executables beside the ruby linked onto PATH.\n" +
		"module Gem\n  def self.default_bindir\n    '" + quoted + "'\n  end\nend\n"
	if err := os.WriteFile(hook, []byte(content), 0644); err != nil { //nolint:gosec // read by Ruby, not secret
		return "", false, err
	}
	return linkedDir, true, nil
}

// RemoveGemHook removes the hook GemExecutables wrote into the install at
// root, if an earlier run left one.
func RemoveGemHook(root, version string) error {
	hook := gemHook(root, version)
	data, err := os.ReadFile(hook) //nolint:gosec // a path inside the install
	if errors.Is(err, os.ErrNotExist) || (err == nil && !strings.HasPrefix(string(data), hookMarker)) {
		return nil
	}
	if err != nil {
		return err
	}
	return os.Remove(hook)
}

// gemHook returns where RubyGems looks for operating_system.rb in the
// install at root: its site directory, named for the version's ABI, which is
// its major.minor.0.
func gemHook(root, version string) string {
	parts := strings.SplitN(strings.SplitN(version, "-", 2)[0], ".", 3)
	abi := parts[0] + "." + parts[1] + ".0"
	return filepath.Join(root, "lib", "ruby", "site_ruby", abi, "rubygems", "defaults", "operating_system.rb")
}

func onSearchPath(dir string) bool {
	for _, d := range pathlink.SearchDirs("") {
		if sameFile(d, dir) {
			return true
		}
	}
	return false
}
