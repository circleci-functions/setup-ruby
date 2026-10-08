package main

import (
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
)

func TestLockfile(t *testing.T) {
	tests := []struct {
		name  string
		files []string
		env   map[string]string
		want  string
	}{
		{name: "Gemfile", files: []string{"Gemfile", "gems.rb"}, want: "Gemfile.lock"},
		{name: "gems.rb", files: []string{"gems.rb"}, want: "gems.locked"},
		{name: "none", want: ""},
		{name: "BUNDLE_GEMFILE", env: map[string]string{"BUNDLE_GEMFILE": "Gemfile.next"}, want: "Gemfile.next.lock"},
		{name: "BUNDLE_GEMFILE gems.rb", env: map[string]string{"BUNDLE_GEMFILE": "sub/gems.rb"}, want: "sub/gems.locked"},
		{name: "BUNDLE_LOCKFILE", files: []string{"Gemfile"}, env: map[string]string{"BUNDLE_LOCKFILE": "other.lock"}, want: "other.lock"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("BUNDLE_GEMFILE", "")
			t.Setenv("BUNDLE_LOCKFILE", "")
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			wd := t.TempDir()
			for _, name := range tc.files {
				assert.NilError(t, os.WriteFile(filepath.Join(wd, name), nil, 0600))
			}
			want := ""
			if tc.want != "" {
				want = filepath.Join(wd, filepath.FromSlash(tc.want))
			}
			assert.Equal(t, lockfile(wd), want)
		})
	}
}
