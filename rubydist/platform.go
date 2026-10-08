// Package rubydist finds, downloads, verifies and unpacks relocatable Ruby
// builds: jdx/ruby's for Linux with glibc and macOS, rv-ruby's for Linux with
// musl, and RubyInstaller2's for Windows, plus the MSYS2 build tools
// RubyInstaller uses to compile native extensions.
package rubydist

import (
	"debug/elf"
	"fmt"
	"io"
	"path/filepath"
	"runtime"
	"strings"
)

// Platform is a build target.
type Platform struct {
	// OS is linux, darwin or windows.
	OS string
	// Arch is x86_64 or aarch64.
	Arch string
	// Libc is gnu or musl on Linux, and none elsewhere.
	Libc string
}

var archFamily = map[string]string{
	"amd64": "x86_64",
	"arm64": "aarch64",
}

// Host returns the platform setup-ruby is running on.
func Host() (Platform, error) {
	return platformFor(runtime.GOOS, runtime.GOARCH, isMusl())
}

func platformFor(goos, goarch string, musl bool) (Platform, error) {
	arch, ok := archFamily[goarch]
	if !ok {
		return Platform{}, fmt.Errorf("unsupported architecture %s", goarch)
	}
	switch goos {
	case "darwin", "windows":
		return Platform{OS: goos, Arch: arch, Libc: "none"}, nil
	case "linux":
		libc := "gnu"
		if musl {
			libc = "musl"
		}
		return Platform{OS: goos, Arch: arch, Libc: libc}, nil
	}
	return Platform{}, fmt.Errorf("unsupported operating system %s", goos)
}

// isMusl reports whether this Linux uses musl rather than glibc, as Alpine
// does, by the dynamic loader /bin/sh asks for. The loader merely existing is
// not enough: a glibc distro can install musl alongside.
func isMusl() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	if interp, ok := interpreter("/bin/sh"); ok {
		return strings.Contains(filepath.Base(interp), "musl")
	}
	matches, _ := filepath.Glob("/lib/ld-musl-*.so.1")
	return len(matches) > 0
}

// interpreter returns the dynamic loader the ELF executable at path names.
func interpreter(path string) (string, bool) {
	f, err := elf.Open(path)
	if err != nil {
		return "", false
	}
	defer func() { _ = f.Close() }()
	for _, prog := range f.Progs {
		if prog.Type != elf.PT_INTERP {
			continue
		}
		data, err := io.ReadAll(prog.Open())
		if err != nil {
			return "", false
		}
		return strings.TrimRight(string(data), "\x00"), true
	}
	return "", false
}

func (p Platform) String() string {
	if p.Libc == "none" {
		return p.OS + "-" + p.Arch
	}
	return p.OS + "-" + p.Arch + "-" + p.Libc
}
