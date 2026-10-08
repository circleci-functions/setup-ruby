// Package fakerbenv writes a stand-in for an rbenv root, so tests can
// register a Ruby install with rbenv and run it through rbenv's shims without
// a real rbenv.
package fakerbenv

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/skip"
)

// New writes an rbenv root whose shims run the version named by
// $RBENV_VERSION, the .ruby-version in the directory they run in, or the
// root's version file, looked up by name as rbenv's are, holding a version of
// its own named preinstalled, whose ruby answers -v. Unlike rbenv, the shims
// do not look for a .ruby-version in parent directories.
func New(t *testing.T, preinstalled string) string {
	t.Helper()
	skip.If(t, runtime.GOOS == "windows", "the fake rbenv is shell scripts")
	root := t.TempDir()
	for _, dir := range []string{"bin", "shims", filepath.Join("versions", preinstalled, "bin")} {
		assert.NilError(t, os.MkdirAll(filepath.Join(root, dir), 0750))
	}
	assert.NilError(t, os.WriteFile(filepath.Join(root, "version"), []byte(preinstalled+"\n"), 0600))
	selected := `if [ -n "$RBENV_VERSION" ]; then v=$RBENV_VERSION; elif [ -f .ruby-version ]; then read -r v < .ruby-version; else read -r v < "` + root + `/version"; fi
[ -d "` + root + `/versions/$v" ] || v=${v#ruby-}`
	scripts := map[string]string{
		"bin/rbenv": "#!/bin/sh\n" + selected + "\ncase \"$1\" in\n" +
			"global) printf '%s\\n' \"$2\" > \"" + root + "/version\" ;;\n" +
			"rehash) : > \"" + root + "/rehashed\" ;;\n" +
			"version) echo \"$v (set by " + root + "/version)\" ;;\n" +
			"esac\n",
		"shims/ruby": "#!/bin/sh\n" + selected + "\nexec \"" + root + "/versions/$v/bin/ruby\" \"$@\"\n",
		filepath.Join("versions", preinstalled, "bin", "ruby"): "#!/bin/sh\necho 'ruby " + preinstalled + " (2024-01-01 revision 0000000000) [fake]'\n",
	}
	for name, content := range scripts {
		assert.NilError(t, os.WriteFile(filepath.Join(root, name), []byte(content), 0700)) //nolint:gosec // must be executable
	}
	return root
}
