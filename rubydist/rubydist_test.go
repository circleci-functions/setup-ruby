package rubydist_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/skip"

	"github.com/circleci-functions/setup-ruby/internal/testing/fakedist"
	"github.com/circleci-functions/setup-ruby/rubydist"
)

func TestSnapshot_CoversEveryPlatform(t *testing.T) {
	c, err := rubydist.Snapshot()
	assert.NilError(t, err)

	windows := rubydist.Platform{OS: "windows", Arch: "x86_64", Libc: "none"}
	for _, p := range []rubydist.Platform{
		{OS: "linux", Arch: "x86_64", Libc: "gnu"},
		{OS: "linux", Arch: "aarch64", Libc: "gnu"},
		{OS: "linux", Arch: "x86_64", Libc: "musl"},
		{OS: "linux", Arch: "aarch64", Libc: "musl"},
		{OS: "darwin", Arch: "aarch64", Libc: "none"},
		windows,
	} {
		t.Run(p.String(), func(t *testing.T) {
			versions := c.Versions(p)
			assert.Assert(t, len(versions) > 0, "no Ruby builds for %s", p)
			b, ok := c.Find(p, versions[len(versions)-1])
			assert.Assert(t, ok)
			assert.Assert(t, len(b.SHA256) == 64, "build has no SHA-256: %+v", b)
		})
	}
	_, ok := c.Toolchain(windows)
	assert.Assert(t, ok, "no MSYS2 toolchain for %s", windows)
}

func TestCatalog_Versions(t *testing.T) {
	linux := rubydist.Platform{OS: "linux", Arch: "x86_64", Libc: "gnu"}
	c := rubydist.Catalog{Rubies: []rubydist.Build{
		{Version: "3.4.10", Platform: linux.String()},
		{Version: "3.4.9", Platform: linux.String()},
		{Version: "3.4.0-rc1", Platform: linux.String()},
		{Version: "3.4.0", Platform: linux.String()},
		{Version: "3.5.0", Platform: "darwin-aarch64"},
	}}
	assert.DeepEqual(t, c.Versions(linux), []string{"3.4.0-rc1", "3.4.0", "3.4.9", "3.4.10"})
}

func TestCatalog_MarshalRoundTrips(t *testing.T) {
	c := rubydist.Catalog{
		Rubies:     []rubydist.Build{{Version: "3.4.1", Platform: "linux-x86_64-gnu", URL: "u", SHA256: "s", Root: "ruby-3.4.1"}},
		Toolchains: []rubydist.Build{{Version: "v1", Platform: "windows-x86_64", URL: "t", SHA256: "s"}},
	}
	data, err := c.Marshal()
	assert.NilError(t, err)
	got, err := rubydist.Parse(data)
	assert.NilError(t, err)
	assert.DeepEqual(t, got, c)
}

func TestInstall(t *testing.T) {
	skipWithoutBuilds(t)
	rel := fakedist.Release{Version: "3.4.1"}
	srv := fakedist.New(t, rel)
	dest := filepath.Join(t.TempDir(), "ruby")
	assert.NilError(t, os.MkdirAll(filepath.Join(dest, "stale"), 0750))

	assert.NilError(t, rubydist.Install(context.Background(), srv.Build(rel), dest))

	_, err := os.Stat(filepath.Join(dest, "bin", "ruby"))
	assert.NilError(t, err, "the archive's root should be unpacked at dest")
	_, err = os.Stat(filepath.Join(dest, "stale"))
	assert.Assert(t, os.IsNotExist(err), "an earlier install at dest should be replaced")
}

func TestInstall_ChecksumMismatch(t *testing.T) {
	skipWithoutBuilds(t)
	rel := fakedist.Release{Version: "3.4.1"}
	srv := fakedist.New(t, rel)
	srv.Corrupt()
	dest := filepath.Join(t.TempDir(), "ruby")

	err := rubydist.Install(context.Background(), srv.Build(rel), dest)
	assert.ErrorContains(t, err, "checksum mismatch")
	_, err = os.Stat(dest)
	assert.Assert(t, os.IsNotExist(err), "nothing should be installed from a corrupt archive")
}

func TestLive_Find(t *testing.T) {
	tests := []struct {
		name     string
		platform rubydist.Platform
	}{
		{name: "provenance", platform: rubydist.Platform{OS: "linux", Arch: "x86_64", Libc: "gnu"}},
		{name: "checksum list", platform: rubydist.Platform{OS: "linux", Arch: "aarch64", Libc: "musl"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rel := fakedist.Release{Version: "3.9.1", Platform: tc.platform, Unlisted: true}
			srv := fakedist.New(t, rel, fakedist.Release{Version: "3.9.0", Platform: tc.platform, Unlisted: true})

			got, err := srv.Live().Find(context.Background(), tc.platform, "3.9.1")
			assert.NilError(t, err)
			assert.DeepEqual(t, got, srv.Build(rel))
		})
	}
}

func TestLive_Find_Errors(t *testing.T) {
	linux := rubydist.Platform{OS: "linux", Arch: "x86_64", Libc: "gnu"}
	srv := fakedist.New(t, fakedist.Release{Version: "3.9.1", Platform: linux, Unlisted: true})

	_, err := srv.Live().Find(context.Background(), linux, "3.9.2")
	assert.ErrorContains(t, err, "3.9.2.intoto.jsonl")

	windows := rubydist.Platform{OS: "windows", Arch: "x86_64", Libc: "none"}
	_, err = srv.Live().Find(context.Background(), windows, "3.9.1")
	assert.ErrorContains(t, err, "no published list of builds for windows-x86_64")
}

// skipWithoutBuilds skips on Windows, where the fake builds' tar.gz archives
// stand in for RubyInstaller's 7z ones.
func skipWithoutBuilds(t *testing.T) {
	t.Helper()
	skip.If(t, runtime.GOOS == "windows", "the fake builds are tar.gz archives")
}
