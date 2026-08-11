package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/circleci-functions/setup-ruby/ruby"
)

type rubocopCheckCmd struct {
	AppDir      string `help:"Directory containing the Gemfile." default:"."`
	CheckPath   string `help:"Path RuboCop inspects." default:"."`
	Format      string `help:"RuboCop formatter." default:"progress"`
	OutFileName string `help:"Output file name." default:"check-results.xml"`
	OutPath     string `help:"Directory for the output file." default:"/tmp/rubocop-results"`
	Parallel    bool   `help:"Inspect files in parallel." default:"true" negatable:""`
}

func (c *rubocopCheckCmd) Run(ctx context.Context) error {
	appDir, err := filepath.Abs(c.AppDir)
	if err != nil {
		return fmt.Errorf("resolve app-dir: %w", err)
	}

	outPath := c.OutPath
	if !filepath.IsAbs(outPath) {
		outPath = filepath.Join(appDir, outPath)
	}
	if err := os.MkdirAll(outPath, 0o755); err != nil { //nolint:gosec // results dir mirrors the orb's mkdir -p
		return fmt.Errorf("create output directory %q: %w", outPath, err)
	}

	version, _ := ruby.Output(ctx, appDir, "bundle", "exec", "rubocop", "-v")
	outFile := filepath.Join(outPath, c.OutFileName)
	args := ruby.RubocopArgs(c.CheckPath, c.Format, outFile, c.Parallel, version)
	return ruby.Run(ctx, appDir, "bundle", args...)
}
