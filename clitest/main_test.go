// Package clitest_test exercises the setup-ruby CLI contract end to end
// against fakes: a fake CircleCI cache server, a fake server of Ruby builds
// whose executables are shell scripts, and stub bundle and gem executables.
// It is hermetic -- no real Ruby and no network. Real installs are covered by
// acceptance/.
package clitest_test

import (
	"context"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/circleci-functions/toolkit/sdk/sdktest"
	"github.com/circleci-functions/toolkit/testing/compiler"
	"github.com/circleci-functions/toolkit/testing/runner"
)

var binaryPath string

func TestMain(m *testing.M) {
	status, err := runTests(m)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	os.Exit(status)
}

func runTests(m *testing.M) (int, error) {
	ctx := context.Background()

	p := compiler.NewParallel(1)
	defer p.Cleanup()

	p.Add(compiler.Work{
		Result: &binaryPath,
		Name:   "setup-ruby",
		Target: "..",
		Source: ".",
	})

	if err := p.Run(ctx); err != nil {
		return 0, err
	}

	return m.Run(), nil
}

func run(t *testing.T, extraEnv map[string]string, args ...string) (string, error) {
	t.Helper()
	r := runner.New(t, binaryPath, runner.WithUnsafeOSEnvVars(), runner.WithUnsafeAdditionalEnvVars(extraEnv))
	_, err := r.Run(args...)
	return r.StdoutString() + r.StderrString(), err
}

// runIn runs the binary in s's environment, with extraEnv applied on top and
// dir as its working directory. The sandbox keeps an install out of the
// developer's real home directory and away from any cache it did not set up.
func runIn(t *testing.T, s *sdktest.Sandbox, dir string, extraEnv map[string]string, args ...string) (string, error) {
	t.Helper()
	env := s.Env()
	maps.Copy(env, extraEnv)
	opts := []runner.Opt{runner.WithSandboxEnv(env)}
	if dir != "" {
		opts = append(opts, runner.WithCmdDir(dir))
	}
	r := runner.New(t, binaryPath, opts...)
	_, err := r.Run(args...)
	return r.StdoutString() + r.StderrString(), err
}

const appFixture = "testdata/app"

func copyFixture(t *testing.T, dst string) {
	t.Helper()
	entries, err := os.ReadDir(appFixture)
	assert.NilError(t, err)
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(appFixture, e.Name())) //nolint:gosec // fixture path
		assert.NilError(t, err)
		assert.NilError(t, os.WriteFile(filepath.Join(dst, e.Name()), data, 0o600))
	}
}

// fakeRubyToolsPath writes stub `bundle` and `gem` executables into a temp dir
// and returns it, so the install can be exercised without a real Ruby or the
// network. The stub `bundle` fails the `config set` probe (selecting the modern
// form), reports a version, fails `check` (so install runs) and otherwise
// succeeds, creating the vendor/bundle directory on install.
func fakeRubyToolsPath(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	bundle := `#!/bin/sh
if [ "$1" = "config" ] && [ "$2" = "set" ] && [ "$#" -eq 2 ]; then
  exit 1
fi
case "$1" in
  version) echo "Bundler version 2.4.10" ;;
  check) exit 1 ;;
  install) mkdir -p vendor/bundle ;;
esac
exit 0
`
	gem := "#!/bin/sh\nexit 0\n"

	assert.NilError(t, os.WriteFile(filepath.Join(dir, "bundle"), []byte(bundle), 0o700)) //nolint:gosec // test stub
	assert.NilError(t, os.WriteFile(filepath.Join(dir, "gem"), []byte(gem), 0o700))       //nolint:gosec // test stub
	return dir
}
