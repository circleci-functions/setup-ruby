package clitest_test

import (
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

func TestCLI_Describe(t *testing.T) {
	out, err := run(t, nil, "--describe")
	assert.NilError(t, err, "output: %s", out)

	for _, want := range []string{"name: setup-ruby", "install-gems"} {
		assert.Assert(t, strings.Contains(out, want), "descriptor missing %q; got: %s", want, out)
	}
}

func TestCLI_Help(t *testing.T) {
	tests := []struct {
		args []string
		want []string
	}{
		{args: []string{"install", "--help"}, want: []string{"--ruby-version", "--ruby-version-file", "--working-directory", "--rubygems", "--bundler", "--windows-toolchain", "--cache"}},
		{args: []string{"install-gems", "--help"}, want: []string{"--app-dir", "--gemfile", "--key"}},
	}
	for _, tc := range tests {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			out, err := run(t, nil, tc.args...)
			assert.NilError(t, err, "output: %s", out)
			for _, flag := range tc.want {
				assert.Assert(t, strings.Contains(out, flag), "help missing %s; got: %s", flag, out)
			}
			for _, hidden := range []string{"--install-dir", "--bin-dir", "--cache-nonce", "--downloads-url", "--bundler-version"} {
				assert.Assert(t, !strings.Contains(out, hidden), "help shows %s; got: %s", hidden, out)
			}
		})
	}
}

func TestCLI_UnknownFlag(t *testing.T) {
	_, err := run(t, nil, "install-gems", "--no-such-flag")
	assert.Assert(t, err != nil, "expected non-zero exit for unknown flag")
}

func TestCLI_AppDirMissing(t *testing.T) {
	out, err := run(t, nil, "install-gems", "--app-dir", "/nonexistent-dir-xyz/app", "--cache=false")
	assert.Assert(t, err != nil, "expected non-zero exit; output: %s", out)
	assert.Assert(t, strings.Contains(out, "app-dir"), "expected app-dir error; got: %s", out)
}
