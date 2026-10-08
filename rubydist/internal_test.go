package rubydist

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
)

func TestPlatformFor(t *testing.T) {
	tests := []struct {
		goos, goarch string
		musl         bool
		want         string
	}{
		{goos: "linux", goarch: "amd64", want: "linux-x86_64-gnu"},
		{goos: "linux", goarch: "arm64", musl: true, want: "linux-aarch64-musl"},
		{goos: "darwin", goarch: "arm64", want: "darwin-aarch64"},
		{goos: "windows", goarch: "amd64", want: "windows-x86_64"},
	}
	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			p, err := platformFor(tc.goos, tc.goarch, tc.musl)
			assert.NilError(t, err)
			assert.Equal(t, p.String(), tc.want)
		})
	}

	_, err := platformFor("linux", "s390x", false)
	assert.ErrorContains(t, err, "unsupported architecture s390x")
	_, err = platformFor("freebsd", "amd64", false)
	assert.ErrorContains(t, err, "unsupported operating system freebsd")
}

func TestExtract7z(t *testing.T) {
	dest := t.TempDir()
	assert.NilError(t, extract7z(filepath.Join("testdata", "rubyinstaller.7z"), dest))

	data, err := os.ReadFile(filepath.Join(dest, "rubyinstaller-3.4.1-1-x64", "bin", "ruby.exe"))
	assert.NilError(t, err)
	assert.Equal(t, string(data), "@echo ruby 3.4.1\r\n")
}

func TestExtractTarGz_RejectsEscapes(t *testing.T) {
	tests := []struct {
		name string
		hdr  tar.Header
	}{
		{name: "parent path", hdr: tar.Header{Name: "../evil", Typeflag: tar.TypeReg}},
		{name: "symlink out", hdr: tar.Header{Name: "ruby/link", Typeflag: tar.TypeSymlink, Linkname: "../../evil"}},
		{name: "absolute symlink", hdr: tar.Header{Name: "ruby/link", Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			gz := gzip.NewWriter(&buf)
			tw := tar.NewWriter(gz)
			assert.NilError(t, tw.WriteHeader(&tc.hdr))
			assert.NilError(t, tw.Close())
			assert.NilError(t, gz.Close())
			archive := filepath.Join(t.TempDir(), "evil.tar.gz")
			assert.NilError(t, os.WriteFile(archive, buf.Bytes(), 0600))

			err := extractTarGz(archive, t.TempDir())
			assert.Assert(t, err != nil, "an entry escaping the destination must fail")
		})
	}
}
