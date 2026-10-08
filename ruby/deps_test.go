package ruby

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestLockedBundler(t *testing.T) {
	tests := []struct {
		name     string
		lockfile string
		want     string
	}{
		{name: "bundled with", lockfile: "GEM\n  remote: https://rubygems.org/\n\nBUNDLED WITH\n   2.4.10\n", want: "2.4.10"},
		{name: "trailing blank lines", lockfile: "BUNDLED WITH\n   2.3.1\n\n\n", want: "2.3.1"},
		{name: "crlf", lockfile: "BUNDLED WITH\r\n   2.6.2\r\n", want: "2.6.2"},
		{name: "no bundled with", lockfile: "DEPENDENCIES\n  rake\n", want: ""},
		{name: "empty", lockfile: "", want: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, LockedBundler(tc.lockfile), tc.want)
		})
	}
}

func TestBundleConfigCommands(t *testing.T) {
	tests := []struct {
		name   string
		cfg    BundleConfig
		useSet bool
		want   [][]string
	}{
		{
			name:   "deployment set form",
			cfg:    BundleConfig{Deployment: true, GemfileAbs: "/app/Gemfile", Path: DefaultPath},
			useSet: true,
			want: [][]string{
				{"config", "set", "deployment", "true"},
				{"config", "set", "gemfile", "/app/Gemfile"},
				{"config", "set", "path", "./vendor/bundle"},
			},
		},
		{
			name:   "no deployment custom path old form",
			cfg:    BundleConfig{GemfileAbs: "/app/Gemfile", Path: "/custom"},
			useSet: false,
			want: [][]string{
				{"config", "gemfile", "/app/Gemfile"},
				{"config", "path", "/custom"},
			},
		},
		{
			name:   "with openssl dir",
			cfg:    BundleConfig{GemfileAbs: "/app/Gemfile", Path: DefaultPath, Deployment: true, OpenSSLDir: "/usr/local/rvm/usr"},
			useSet: true,
			want: [][]string{
				{"config", "set", "deployment", "true"},
				{"config", "set", "gemfile", "/app/Gemfile"},
				{"config", "set", "path", "./vendor/bundle"},
				{"config", "set", "build.openssl", "--with-openssl-dir=/usr/local/rvm/usr"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.DeepEqual(t, tc.cfg.Commands(tc.useSet), tc.want)
		})
	}
}
