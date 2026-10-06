package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/circleci-functions/toolkit/pathlink"

	"github.com/circleci-functions/setup-ruby/rubypath"
	"github.com/circleci-functions/setup-ruby/rubyversion"
)

// rubyRunner runs the scripts, such as gem, of the Ruby a job uses.
type rubyRunner interface {
	command(ctx context.Context, script string, args ...string) *exec.Cmd
}

// installedRuby runs the scripts of the install at root by absolute path,
// rather than whatever PATH resolves.
type installedRuby struct{ root string }

// command runs script from the install's bin directory. On Windows that holds
// it as a Ruby script beside a .bat wrapper, so ruby.exe runs the script.
func (r installedRuby) command(ctx context.Context, script string, args ...string) *exec.Cmd {
	path := filepath.Join(rubypath.BinDir(r.root), script)
	if isWindows() {
		return exec.CommandContext(ctx, rubypath.Executable(r.root), append([]string{path}, args...)...)
	}
	return exec.CommandContext(ctx, path, args...)
}

// systemRuby runs the scripts PATH resolves.
type systemRuby struct{}

func (systemRuby) command(_ context.Context, script string, args ...string) *exec.Cmd {
	path, err := exec.LookPath(script)
	if err != nil {
		path = script
	}
	return pathlink.Command(path, args...)
}

// runScript runs script with args, streaming its output.
func runScript(ctx context.Context, r rubyRunner, script string, args ...string) error {
	cmd := r.command(ctx, script, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w", script, strings.Join(args, " "), err)
	}
	return nil
}

// scriptOutput runs script with args and returns its trimmed output.
func scriptOutput(ctx context.Context, r rubyRunner, script string, args ...string) (string, error) {
	out, err := r.command(ctx, script, args...).Output()
	if err != nil {
		return "", fmt.Errorf("%s %s: %w", script, strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}

// updateRubygems updates the RubyGems of r as want asks: default leaves it,
// latest updates to the newest, and a version updates to that one unless r
// already has it or newer.
func updateRubygems(ctx context.Context, r rubyRunner, want string) error {
	switch want {
	case "", "default":
		return nil
	case "latest":
		fmt.Fprintln(os.Stderr, "setup-ruby: updating RubyGems to the latest version")
		return runScript(ctx, r, "gem", "update", "--system")
	}
	current, err := scriptOutput(ctx, r, "gem", "--version")
	if err != nil {
		return err
	}
	if rubyversion.Compare(current, want) >= 0 {
		fmt.Fprintf(os.Stderr, "setup-ruby: RubyGems %s is already at least %s\n", current, want)
		return nil
	}
	fmt.Fprintf(os.Stderr, "setup-ruby: updating RubyGems %s to %s\n", current, want)
	return runScript(ctx, r, "gem", "update", "--system", want)
}

func isWindows() bool {
	return runtime.GOOS == "windows"
}
