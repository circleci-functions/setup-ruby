package main

import (
	"context"
	crand "crypto/rand"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"

	"github.com/circleci-functions/setup-ruby/ruby"
)

type rspecTestCmd struct {
	AppDir    string `help:"Directory containing the Gemfile." default:"."`
	Include   string `help:"Glob(s) for test files; comma-separate multiple in a single brace glob." default:"spec/**/*_spec.rb"`
	Order     string `help:"RSpec --order value."`
	Tag       string `help:"Space-separated RSpec tags to run."`
	OutPath   string `help:"Directory for the JUnit results.xml." default:"/tmp/test-results/rspec"`
	RerunFail bool   `help:"Use 'circleci tests run' so failed tests can be rerun." default:"true" negatable:""`
}

func (c *rspecTestCmd) Run(ctx context.Context) error {
	appDir, err := filepath.Abs(c.AppDir)
	if err != nil {
		return fmt.Errorf("resolve app-dir: %w", err)
	}

	if os.Getenv("CIRCLE_NODE_TOTAL") == "1" {
		fmt.Fprintln(os.Stderr, "setup-ruby: job parallelism is 1; splitting by timings needs 2 or more nodes for historical data")
	}

	outPath := c.OutPath
	if !filepath.IsAbs(outPath) {
		outPath = filepath.Join(appDir, outPath)
	}
	if err := os.MkdirAll(outPath, 0o755); err != nil { //nolint:gosec // results dir mirrors the orb's mkdir -p
		return fmt.Errorf("create output directory %q: %w", outPath, err)
	}

	globs := strings.Fields(c.Include)
	extra := ruby.RspecExtraArgs(c.Order, c.Tag)
	seed := randomSeed()

	files, err := ruby.Output(ctx, appDir, "circleci", append([]string{"tests", "glob"}, globs...)...)
	if err != nil {
		return fmt.Errorf("circleci tests glob: %w", err)
	}

	if c.RerunFail {
		cmd := ruby.RspecRerunCommand(outPath, seed, extra)
		return ruby.RunStdin(ctx, appDir, files, "circleci", "tests", "run",
			"--command", cmd, "--verbose", "--split-by=timings")
	}

	split, err := ruby.OutputStdin(ctx, appDir, files, "circleci", "tests", "split", "--split-by=timings")
	if err != nil {
		return fmt.Errorf("circleci tests split: %w", err)
	}
	return ruby.Run(ctx, appDir, "bundle", ruby.RspecArgs(outPath, seed, strings.Fields(split), extra)...)
}

func randomSeed() string {
	n, err := crand.Int(crand.Reader, big.NewInt(1<<16))
	if err != nil {
		return "0"
	}
	return n.String()
}
