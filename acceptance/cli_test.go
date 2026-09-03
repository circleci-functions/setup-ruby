package setupruby_test

import (
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

func TestCLI_Describe(t *testing.T) {
	out, err := run(t, nil, "--describe")
	assert.NilError(t, err, "output: %s", out)

	for _, want := range []string{"name: setup-ruby", "Install Ruby gems with Bundler"} {
		assert.Assert(t, strings.Contains(out, want), "descriptor missing %q; got: %s", want, out)
	}
}

func TestCLI_Help(t *testing.T) {
	out, err := run(t, nil, "--help")
	assert.NilError(t, err, "output: %s", out)
	assert.Assert(t, strings.Contains(out, "--app-dir"), "help missing flags; got: %s", out)
}

func TestCLI_UnknownFlag(t *testing.T) {
	_, err := run(t, nil, "--no-such-flag")
	assert.Assert(t, err != nil, "expected non-zero exit for unknown flag")
}

func TestCLI_AppDirMissing(t *testing.T) {
	out, err := run(t, nil, "--app-dir", "/nonexistent-dir-xyz/app", "--cache=false")
	assert.Assert(t, err != nil, "expected non-zero exit; output: %s", out)
	assert.Assert(t, strings.Contains(out, "app-dir"), "expected app-dir error; got: %s", out)
}
