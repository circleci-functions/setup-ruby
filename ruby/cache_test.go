package ruby

import (
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
)

func TestResolveLockfile(t *testing.T) {
	dir := t.TempDir()
	write := func(name string) {
		assert.NilError(t, os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600))
	}

	tests := []struct {
		name  string
		files []string
		in    LockfileInput
		want  string
		ok    bool
	}{
		{
			name:  "gemfile lock",
			files: []string{"Gemfile.lock"},
			in:    LockfileInput{Dir: dir, Gemfile: "Gemfile"},
			want:  "Gemfile.lock",
			ok:    true,
		},
		{
			name:  "no lockfile",
			files: nil,
			in:    LockfileInput{Dir: dir, Gemfile: "Gemfile"},
			ok:    false,
		},
		{
			name:  "override wins",
			files: []string{"Gemfile.lock", "custom.lock"},
			in:    LockfileInput{Dir: dir, Gemfile: "Gemfile", Override: "custom.lock"},
			want:  "custom.lock",
			ok:    true,
		},
		{
			name:  "override missing falls through",
			files: []string{"Gemfile.lock"},
			in:    LockfileInput{Dir: dir, Gemfile: "Gemfile", Override: "missing.lock"},
			want:  "Gemfile.lock",
			ok:    true,
		},
		{
			name:  "deps next when set and present",
			files: []string{"Gemfile.lock", "Gemfile_next.lock"},
			in:    LockfileInput{Dir: dir, Gemfile: "Gemfile", DepsNext: true},
			want:  "Gemfile_next.lock",
			ok:    true,
		},
		{
			name:  "deps next ignored when file absent",
			files: []string{"Gemfile.lock"},
			in:    LockfileInput{Dir: dir, Gemfile: "Gemfile", DepsNext: true},
			want:  "Gemfile.lock",
			ok:    true,
		},
		{
			name:  "rb gemfile uses locked sibling",
			files: []string{"gems.locked"},
			in:    LockfileInput{Dir: dir, Gemfile: "gems.rb"},
			want:  "gems.locked",
			ok:    true,
		},
		{
			name:  "rb gemfile without locked sibling yields none",
			files: []string{"Gemfile.lock"},
			in:    LockfileInput{Dir: dir, Gemfile: "gems.rb"},
			ok:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, f := range []string{"Gemfile.lock", "Gemfile_next.lock", "custom.lock", "gems.locked"} {
				_ = os.Remove(filepath.Join(dir, f))
			}
			for _, f := range tc.files {
				write(f)
			}
			got, ok := ResolveLockfile(tc.in)
			assert.Equal(t, ok, tc.ok)
			if tc.ok {
				assert.Equal(t, got, filepath.Join(dir, tc.want))
			}
		})
	}
}

func TestResolveLockfileAbsoluteOverride(t *testing.T) {
	dir := t.TempDir()
	abs := filepath.Join(t.TempDir(), "elsewhere.lock")
	assert.NilError(t, os.WriteFile(abs, []byte("x"), 0o600))

	got, ok := ResolveLockfile(LockfileInput{Dir: dir, Gemfile: "Gemfile", Override: abs})
	assert.Assert(t, ok)
	assert.Equal(t, got, abs)
}

func TestChecksum(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "Gemfile.lock")
	assert.NilError(t, os.WriteFile(p, []byte("hello"), 0o600))

	sum, err := Checksum(p)
	assert.NilError(t, err)
	// sha256("hello")
	assert.Equal(t, sum, "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824")

	_, err = Checksum(filepath.Join(dir, "nope.lock"))
	assert.Assert(t, err != nil)
}

func TestCacheKeyParts(t *testing.T) {
	tests := []struct {
		name         string
		parts        CacheKeyParts
		onlyChecksum bool
		wantExact    string
		wantRestore  []string
	}{
		{
			name:        "full key with arch and branch",
			parts:       CacheKeyParts{Prefix: "gems-v1", Arch: "amd64-linux", Branch: "main", Checksum: "abc"},
			wantExact:   "gems-v1-amd64-linux-main-abc",
			wantRestore: []string{"gems-v1-amd64-linux-main-abc", "gems-v1-amd64-linux-main-"},
		},
		{
			name:        "no arch",
			parts:       CacheKeyParts{Prefix: "gems-v1", Branch: "main", Checksum: "abc"},
			wantExact:   "gems-v1-main-abc",
			wantRestore: []string{"gems-v1-main-abc", "gems-v1-main-"},
		},
		{
			name:        "no arch no branch",
			parts:       CacheKeyParts{Prefix: "gems-v1", Checksum: "abc"},
			wantExact:   "gems-v1-abc",
			wantRestore: []string{"gems-v1-abc", "gems-v1-"},
		},
		{
			name:         "only checksum drops fallback",
			parts:        CacheKeyParts{Prefix: "gems-v1", Arch: "amd64-linux", Branch: "main", Checksum: "abc"},
			onlyChecksum: true,
			wantExact:    "gems-v1-amd64-linux-main-abc",
			wantRestore:  []string{"gems-v1-amd64-linux-main-abc"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.parts.Exact(), tc.wantExact)
			assert.DeepEqual(t, tc.parts.RestoreKeys(tc.onlyChecksum), tc.wantRestore)
		})
	}
}
