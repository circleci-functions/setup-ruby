package rubypath

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/circleci-functions/toolkit/pathlink"
)

// hookMarker starts the RubyGems hook setup-ruby writes, so a later run can
// tell its own hook from one the install shipped.
const hookMarker = "# Written by setup-ruby"

// loadShippedHook has setup-ruby's hook load the one the install ships in its
// standard library first, as RubyInstaller's does: RubyGems loads only the
// first operating_system.rb on the load path, and the site directory comes
// first.
const loadShippedHook = `shipped = File.join(RbConfig::CONFIG["rubylibdir"], "rubygems", "defaults", "operating_system.rb")
load shipped if File.exist?(shipped)
`

// absoluteWindowsStubs has gem install's .bat wrappers run this install's
// ruby.exe by absolute path. RubyGems otherwise writes a bare ruby.exe into
// a wrapper outside the install, such as one in the linked directory or in
// a bundle's path, and PATH holds only a shim of that name.
const absoluteWindowsStubs = `module SetupRubyWindowsStubs
  def windows_stub_script(_bindir, _bin_file_name)
    "@ECHO OFF\r\n@\"#{Gem.ruby.tr('/', '\\')}\" \"%~dpn0\" %*\r\n"
  end
end
Gem.pre_install do |_installer|
  Gem::Installer.prepend(SetupRubyWindowsStubs) unless Gem::Installer < SetupRubyWindowsStubs
end
`

// GemExecutables sets where gem install puts the executables of the gems it
// installs into the install at root, and reports that directory and whether
// it is on PATH.
//
// Where it can, it makes that linkedDir, the directory ruby was linked into,
// so they resolve by name like ruby itself. It does that through RubyGems'
// operating_system.rb hook in the install's own site directory, which no
// other Ruby reads. Where $GEM_HOME is set it leaves gem install to put them
// in $GEM_HOME/bin, and where linkedDir needs elevation to write to, in the
// install's bin directory.
func GemExecutables(root, version, linkedDir string) (dir string, onPath bool, err error) {
	if home := os.Getenv("GEM_HOME"); home != "" {
		bin := filepath.Join(home, "bin")
		return bin, onSearchPath(bin), writeGemHook(root, version, "")
	}
	if !pathlink.Writable(linkedDir) {
		return BinDir(root), false, writeGemHook(root, version, "")
	}
	body := "module Gem\n  def self.default_bindir\n    " + rubyString(linkedDir) + "\n  end\nend\n"
	return linkedDir, true, writeGemHook(root, version, body)
}

// writeGemHook writes body as the install's RubyGems hook, after the code
// that keeps the install's own hook working, and on Windows the code that
// keeps gem install's wrappers working off the install's bin directory.
func writeGemHook(root, version, body string) error {
	if runtime.GOOS == "windows" {
		body = absoluteWindowsStubs + body
	}
	if body == "" {
		return RemoveGemHook(root, version)
	}
	hook := gemHook(root, version)
	if err := os.MkdirAll(filepath.Dir(hook), 0750); err != nil {
		return err
	}
	content := hookMarker + ".\n" + loadShippedHook + body
	return os.WriteFile(hook, []byte(content), 0644) //nolint:gosec // read by Ruby, not secret
}

// RemoveGemHook removes the hook setup-ruby wrote into the install at root,
// if an earlier run left one.
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

// rubyString quotes s as a single-quoted Ruby string literal.
func rubyString(s string) string {
	return "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(s) + "'"
}

func onSearchPath(dir string) bool {
	for _, d := range pathlink.SearchDirs("") {
		if sameFile(d, dir) {
			return true
		}
	}
	return false
}
