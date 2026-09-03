package ruby

import (
	"context"
	"os"
	"os/exec"
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
