package setupruby_test

import (
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

func TestCLI_Install_RequiresVersion(t *testing.T) {
	out, err := run(t, nil, "install")
	assert.Assert(t, err != nil, "expected non-zero exit without --version; output: %s", out)
	assert.Assert(t, strings.Contains(out, "version"), "expected a version error; got: %s", out)
}

func TestCLI_Install_Help(t *testing.T) {
	out, err := run(t, nil, "install", "--help")
	assert.NilError(t, err, "output: %s", out)
	assert.Assert(t, strings.Contains(out, "--openssl-path"), "help missing openssl-path; got: %s", out)
}
