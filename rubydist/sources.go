package rubydist

import (
	"fmt"
	"strings"
)

// The GitHub repositories whose releases the builds come from.
const (
	JDXRepo           = "jdx/ruby"
	RVRepo            = "spinel-coop/rv-ruby"
	RubyInstallerRepo = "oneclick/rubyinstaller2"
	MSYS2Repo         = "ruby/setup-msys2-gcc"
)

// JDXAsset returns the name of jdx/ruby's archive of version for p, which
// holds the install under JDXRoot, or false when it publishes none for p.
func JDXAsset(version string, p Platform) (string, bool) {
	name, ok := map[string]string{
		"linux-x86_64-gnu":  "x86_64_linux",
		"linux-aarch64-gnu": "arm64_linux",
		"darwin-aarch64":    "macos",
	}[p.String()]
	if !ok {
		return "", false
	}
	return fmt.Sprintf("ruby-%s.%s.tar.gz", version, name), true
}

// JDXRoot is the directory in jdx/ruby's archive of version holding the
// install.
func JDXRoot(version string) string {
	return "ruby-" + version
}

// RVAsset returns the name of rv-ruby's archive of version for p, which
// holds the install under RVRoot. setup-ruby takes only its musl builds:
// its glibc builds need a newer glibc than jdx/ruby's.
func RVAsset(version string, p Platform) (string, bool) {
	name, ok := map[string]string{
		"linux-x86_64-musl":  "x86_64_linux_musl",
		"linux-aarch64-musl": "arm64_linux_musl",
	}[p.String()]
	if !ok {
		return "", false
	}
	return fmt.Sprintf("ruby-%s.%s.tar.gz", version, name), true
}

// RVRoot is the directory in rv-ruby's archive of version holding the
// install.
func RVRoot(version string) string {
	return "rv-ruby@" + version + "/" + version
}

// RubyInstallerAsset returns the name of RubyInstaller2's archive of
// version, in the given packaging revision, for p.
func RubyInstallerAsset(version, revision string, p Platform) (string, bool) {
	arch, ok := map[string]string{
		"windows-x86_64":  "x64",
		"windows-aarch64": "arm",
	}[p.String()]
	if !ok {
		return "", false
	}
	return fmt.Sprintf("rubyinstaller-%s-%s-%s.7z", version, revision, arch), true
}

// RubyInstallerRoot is the directory in a RubyInstaller2 archive holding the
// install: the archive's name without its extension.
func RubyInstallerRoot(asset string) string {
	return strings.TrimSuffix(asset, ".7z")
}

// MSYS2Asset returns the name of the MSYS2 archive with the build tools
// RubyInstaller uses for p. The archive's top level is the MSYS2 root.
func MSYS2Asset(p Platform) (string, bool) {
	name, ok := map[string]string{
		"windows-x86_64":  "msys2-ucrt64.7z",
		"windows-aarch64": "msys2-clangarm64.7z",
	}[p.String()]
	return name, ok
}

// ReleaseURL returns the download URL of asset in repo's release tag.
func ReleaseURL(repo, tag, asset string) string {
	return fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repo, tag, asset)
}
