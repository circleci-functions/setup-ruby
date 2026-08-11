package ruby

import (
	"context"
	"os"
	"os/exec"
	"strings"
)

// Run runs name with args in dir, streaming stdout and stderr to the process's
// own streams.
func Run(ctx context.Context, dir, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Output runs name with args in dir and returns its combined stdout+stderr.
func Output(ctx context.Context, dir, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// Succeeds reports whether name with args exits zero in dir, discarding output.
func Succeeds(ctx context.Context, dir, name string, args ...string) bool {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	return cmd.Run() == nil
}

// RunShell runs script with `bash -c` in dir, streaming stdout and stderr. It
// is used only for the few steps that are irreducibly shell pipelines.
func RunShell(ctx context.Context, dir, script string) error {
	cmd := exec.CommandContext(ctx, "bash", "-c", script)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// RunStdin runs name with args in dir, feeding stdin to the process and
// streaming stdout and stderr.
func RunStdin(ctx context.Context, dir, stdin, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(stdin)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// OutputStdin runs name with args in dir, feeding stdin, and returns stdout.
func OutputStdin(ctx context.Context, dir, stdin, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(stdin)
	out, err := cmd.Output()
	return string(out), err
}
