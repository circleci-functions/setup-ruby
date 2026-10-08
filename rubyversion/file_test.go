package rubyversion

import (
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
)

func TestFromFile(t *testing.T) {
	tests := []struct {
		name    string
		file    string
		content string
		want    string
	}{
		{name: "ruby-version", file: ".ruby-version", content: "3.4.1\n", want: "3.4.1"},
		{name: "ruby-version with prefix", file: ".ruby-version", content: "ruby-3.3.0\n", want: "ruby-3.3.0"},
		{name: "ruby-version comments and blanks", file: ".ruby-version", content: "# pinned\n\n  3.4  \n3.3\n", want: "3.4"},
		{name: "ruby-version crlf", file: ".ruby-version", content: "3.4\r\n", want: "3.4"},
		{name: "ruby-version byte order mark", file: ".ruby-version", content: "\ufeff3.4\n", want: "3.4"},
		{name: "any other name", file: "ruby.txt", content: "3.2\n", want: "3.2"},
		{name: "tool-versions", file: ".tool-versions", content: "nodejs 22.1.0\nruby 3.3.6 3.2.2\n", want: "3.3.6"},
		{name: "mise string", file: "mise.toml", content: "[tools]\nnode = \"22\"\nruby = \"3.4\"\n", want: "3.4"},
		{name: "mise list", file: "mise.toml", content: "[tools]\nruby = [\"3.4.1\", \"3.3\"]\n", want: "3.4.1"},
		{name: "mise table", file: ".mise.toml", content: "[tools.ruby]\nversion = \"3.3\"\n", want: "3.3"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), tc.file)
			assert.NilError(t, os.WriteFile(path, []byte(tc.content), 0600))
			got, err := FromFile(path)
			assert.NilError(t, err)
			assert.Equal(t, got, tc.want)
		})
	}
}

func TestFromFile_Errors(t *testing.T) {
	tests := []struct {
		name    string
		file    string
		content string
		wantErr string
	}{
		{name: "empty", file: ".ruby-version", content: "# nothing\n", wantErr: "no Ruby version found"},
		{name: "tool-versions without ruby", file: ".tool-versions", content: "nodejs 22\n", wantErr: "no Ruby version found"},
		{name: "mise without ruby", file: "mise.toml", content: "[tools]\nnode = \"22\"\n", wantErr: "no Ruby version found"},
		{name: "bad toml", file: "mise.toml", content: "[tools\n", wantErr: "mise.toml"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), tc.file)
			assert.NilError(t, os.WriteFile(path, []byte(tc.content), 0600))
			_, err := FromFile(path)
			assert.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestDefault(t *testing.T) {
	tests := []struct {
		name       string
		files      map[string]string
		want       string
		wantSource string
	}{
		{name: "none"},
		{
			name:       "ruby-version first",
			files:      map[string]string{".ruby-version": "3.4\n", ".tool-versions": "ruby 3.3\n", "mise.toml": "[tools]\nruby = \"3.2\"\n"},
			want:       "3.4",
			wantSource: ".ruby-version",
		},
		{
			name:       "tool-versions next",
			files:      map[string]string{".tool-versions": "ruby 3.3\n", "mise.toml": "[tools]\nruby = \"3.2\"\n"},
			want:       "3.3",
			wantSource: ".tool-versions",
		},
		{
			name:       "tool-versions without ruby falls through",
			files:      map[string]string{".tool-versions": "nodejs 22\n", "mise.toml": "[tools]\nruby = \"3.2\"\n"},
			want:       "3.2",
			wantSource: "mise.toml",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, content := range tc.files {
				assert.NilError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0600))
			}
			spec, source, err := Default(dir)
			assert.NilError(t, err)
			assert.Equal(t, spec, tc.want)
			if tc.wantSource == "" {
				assert.Equal(t, source, "")
			} else {
				assert.Equal(t, source, filepath.Join(dir, tc.wantSource))
			}
		})
	}
}
