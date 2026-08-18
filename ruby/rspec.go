package ruby

import (
	"fmt"
	"strings"
)

// RspecTagArgs splits a space-separated tag string into repeated --tag args.
func RspecTagArgs(tag string) []string {
	var args []string
	for _, t := range strings.Fields(tag) {
		args = append(args, "--tag", t)
	}
	return args
}

// RspecExtraArgs returns the optional rspec args built from order and tag.
func RspecExtraArgs(order, tag string) []string {
	var args []string
	if order != "" {
		args = append(args, "--order", order)
	}
	return append(args, RspecTagArgs(tag)...)
}

// RspecRerunCommand returns the command string passed to
// `circleci tests run --command`, which runs rspec over the file list it
// receives on stdin via xargs.
func RspecRerunCommand(outPath, seed string, extra []string) string {
	cmd := fmt.Sprintf(
		`xargs -t bundle exec rspec --seed "%s" --profile 10 --format RspecJunitFormatter --out "%s"/results.xml --format progress`,
		seed, outPath,
	)
	if len(extra) > 0 {
		cmd += " " + strings.Join(extra, " ")
	}
	return cmd
}

// RspecArgs returns the `bundle` args to run rspec directly over files (the
// non-rerun path).
func RspecArgs(outPath, seed string, files, extra []string) []string {
	args := []string{"exec", "rspec", "--seed", seed}
	args = append(args, files...)
	args = append(args, "--profile", "10", "--format", "RspecJunitFormatter", "--out", outPath+"/results.xml", "--format", "progress")
	return append(args, extra...)
}
