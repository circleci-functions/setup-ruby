package setupruby_test

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
	out, err := run(t, nil, "install-gems", "--help")
	assert.NilError(t, err, "output: %s", out)
	assert.Assert(t, strings.Contains(out, "--app-dir"), "help missing flags; got: %s", out)
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
