//go:build acceptance

// Package setupruby_test exercises setup-ruby against real infrastructure:
// real Ruby downloads, real gem installs, and a real CircleCI task-agent cache
// when running inside a job. Tagged out of `task test` because it needs the
// network and (for a live cache) a job. The CLI contract itself is covered
// hermetically by clitest/.
package setupruby_test

import (
	"context"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/circleci-functions/toolkit/pathlink"
	"github.com/circleci-functions/toolkit/sdk/sdktest"
	"github.com/circleci-functions/toolkit/testing/compiler"
	"github.com/circleci-functions/toolkit/testing/runner"
)

// binaryPathEnvVar names a prebuilt setup-ruby to test, for an executor with
// no Go toolchain to compile one with. The smoke jobs set it, and pair it with
// a cross-compiled copy of this suite. A relative path is resolved here, since
// the suite runs the binary from a working directory of its own.
const binaryPathEnvVar = "SETUP_RUBY_BINARY"

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
	if bin := os.Getenv(binaryPathEnvVar); bin != "" {
		abs, err := filepath.Abs(bin)
		if err != nil {
			return 0, fmt.Errorf("%s=%q: %w", binaryPathEnvVar, bin, err)
		}
		if _, err := os.Stat(abs); err != nil {
			return 0, fmt.Errorf("%s=%q: %w", binaryPathEnvVar, bin, err)
		}
		binaryPath = abs
		return m.Run(), nil
	}

	p := compiler.NewParallel(1)
	defer p.Cleanup()

	p.Add(compiler.Work{
		Result: &binaryPath,
		Name:   "setup-ruby",
		Target: "..",
		Source: ".",
	})

	if err := p.Run(context.Background()); err != nil {
		return 0, err
	}
	return m.Run(), nil
}

// acceptanceVersion returns the Ruby version spec to install. CI sets
// SETUP_RUBY_ACCEPTANCE_VERSION from the pipeline's ruby-version parameter,
// so every job in a pipeline installs the same release.
func acceptanceVersion() string {
	if v := os.Getenv("SETUP_RUBY_ACCEPTANCE_VERSION"); v != "" {
		return v
	}
	return "3.4"
}

// runIn runs the binary in s's environment, with extraEnv applied on top and
// dir, when set, as its working directory, and fails the test if it fails.
//
// The cache is CircleCI's own when s was made WithLiveCache and the caller
// asks for it with TOOLKIT_CACHE_MODE=live, and in-memory otherwise, so the
// same suite runs on a laptop.
func runIn(t *testing.T, s *sdktest.Sandbox, dir string, extraEnv map[string]string, args ...string) string {
	t.Helper()
	env := s.Env()
	maps.Copy(env, extraEnv)
	opts := []runner.Opt{runner.WithSandboxEnv(env)}
	if dir != "" {
		opts = append(opts, runner.WithCmdDir(dir))
	}
	r := runner.New(t, binaryPath, opts...)
	_, err := r.Run(args...)
	out := r.StdoutString() + r.StderrString()
	assert.NilError(t, err, "setup-ruby %s: %s", strings.Join(args, " "), out)
	return out
}

// tool returns how to run name from a directory setup-ruby linked into: by
// its own name on Unix, and through the .bat shim on Windows.
func tool(dir, name string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(dir, name+".bat")
	}
	return filepath.Join(dir, name)
}

// runTool runs name from dir with args in s's environment, with extraEnv
// applied on top, in workDir when set, and returns its trimmed output.
func runTool(t *testing.T, s *sdktest.Sandbox, workDir string, extraEnv map[string]string, dir, name string, args ...string) string {
	t.Helper()
	cmd := pathlink.Command(tool(dir, name), args...)
	cmd.Env = s.CmdEnv(os.Environ())
	for k, v := range extraEnv {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	cmd.Dir = workDir
	out, err := cmd.CombinedOutput()
	assert.NilError(t, err, "%s %s: %s", name, strings.Join(args, " "), out)
	return strings.TrimSpace(string(out))
}

// laterStepIn runs name with args in dir in a fresh process, resolving name
// through PATH, as a later step of the job would.
func laterStepIn(t *testing.T, dir, name string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), name, args...) //nolint:gosec // the caller names the command
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// writeApp writes a project depending on rake into a fresh directory.
func writeApp(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	assert.NilError(t, os.WriteFile(filepath.Join(dir, "Gemfile"), []byte("source \"https://rubygems.org\"\ngem \"rake\", \"13.2.1\"\n"), 0600))
	return dir
}
