package ruby

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestBundlerVersion(t *testing.T) {
	lock := "GEM\n  remote: https://rubygems.org/\n\nBUNDLED WITH\n   2.4.10\n"

	tests := []struct {
		name     string
		lockfile string
		override string
		want     string
	}{
		{name: "from lockfile last line", lockfile: lock, want: "2.4.10"},
		{name: "override wins", lockfile: lock, override: "2.5.0", want: "2.5.0"},
		{name: "empty lockfile", lockfile: "", want: ""},
		{name: "trailing blank lines", lockfile: "BUNDLED WITH\n   2.3.1\n\n\n", want: "2.3.1"},
		{name: "no lockfile but override", lockfile: "", override: "2.6.2", want: "2.6.2"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, BundlerVersion(tc.lockfile, tc.override), tc.want)
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
