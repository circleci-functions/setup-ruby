package ruby

import (
	"os"
	"path/filepath"
	"strings"
)

// DefaultPath is bundler's install path that triggers deployment mode.
const DefaultPath = "./vendor/bundle"

// BundlerVersion returns the Bundler version to install. override wins when
// non-empty; otherwise it is the last non-empty line of the lock file (the
// "BUNDLED WITH" version), or "" when the lock file has none.
func BundlerVersion(lockfile, override string) string {
	if override != "" {
		return override
	}
	lines := strings.Split(lockfile, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if v := strings.TrimSpace(lines[i]); v != "" {
			return v
		}
	}
	return ""
}

// BundleConfig describes the `bundle config` settings applied before install.
type BundleConfig struct {
	// Deployment enables bundler's deployment mode.
	Deployment bool
	// GemfileAbs is the absolute path to the Gemfile.
	GemfileAbs string
	// Path is the gem install path.
	Path string
	// OpenSSLDir, when non-empty, configures build.openssl --with-openssl-dir.
	OpenSSLDir string
}

// Commands returns the ordered `bundle` argument lists that apply the config.
// useSet selects the modern `config set` form over the deprecated `config` one.
func (c BundleConfig) Commands(useSet bool) [][]string {
	prefix := []string{"config"}
	if useSet {
		prefix = []string{"config", "set"}
	}
	cmd := func(args ...string) []string {
		return append(append([]string{}, prefix...), args...)
	}

	var cmds [][]string
	if c.Deployment {
		cmds = append(cmds, cmd("deployment", "true"))
	}
	cmds = append(cmds, cmd("gemfile", c.GemfileAbs))
	cmds = append(cmds, cmd("path", c.Path))
	if c.OpenSSLDir != "" {
		cmds = append(cmds, cmd("build.openssl", "--with-openssl-dir="+c.OpenSSLDir))
	}
	return cmds
}

// RVMOpenSSLDir returns the directory to pass to bundler's build.openssl config
// when an RVM install ships its own OpenSSL, or "" when none is detected.
func RVMOpenSSLDir() string {
	home := RVMHome()
	if isDir(filepath.Join(home, "usr", "ssl")) {
		return filepath.Join(home, "usr")
	}
	return ""
}

// RVMHome returns the RVM installation directory, preferring the machine-image
// location, then a per-user install, then the system default.
func RVMHome() string {
	if isDir("/opt/circleci/.rvm") {
		return "/opt/circleci/.rvm"
	}
	if home := filepath.Join(os.Getenv("HOME"), ".rvm"); exists(filepath.Join(home, "scripts", "rvm")) {
		return home
	}
	return "/usr/local/rvm"
}
