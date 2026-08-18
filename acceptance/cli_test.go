package setupruby_test

import (
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

func TestCLI_Describe(t *testing.T) {
	out, err := run(t, nil, "--describe")
	assert.NilError(t, err, "output: %s", out)

	for _, want := range []string{
		"install-deps", "rspec-test", "rubocop-check", "install",
		"with-cache", "include-arch-in-cache-key", "only-use-gemfile-checksum-cache-key",
		"rerun-fail", "parallel", "openssl-path",
	} {
		assert.Assert(t, strings.Contains(out, want), "descriptor missing %q; got: %s", want, out)
	}
}

func TestCLI_Help(t *testing.T) {
	out, err := run(t, nil, "--help")
	assert.NilError(t, err, "output: %s", out)
	assert.Assert(t, strings.Contains(out, "install-deps"), "help missing subcommands; got: %s", out)
}

func TestCLI_UnknownCommand(t *testing.T) {
	_, err := run(t, nil, "not-a-command")
	assert.Assert(t, err != nil, "expected non-zero exit for unknown command")
}

func TestCLI_UnknownFlag(t *testing.T) {
	_, err := run(t, nil, "install-deps", "--no-such-flag")
	assert.Assert(t, err != nil, "expected non-zero exit for unknown flag")
}

func TestCLI_InstallDeps_AppDirMissing(t *testing.T) {
	out, err := run(t, nil, "install-deps", "--app-dir", "/nonexistent-dir-xyz/app", "--no-with-cache")
	assert.Assert(t, err != nil, "expected non-zero exit; output: %s", out)
	assert.Assert(t, strings.Contains(out, "app-dir"), "expected app-dir error; got: %s", out)
}
