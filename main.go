// Command setup-ruby installs Ruby and, through its install-gems subcommand, a
// project's gems with Bundler, caching what each downloads.
//
// Supported platforms: linux, darwin, and windows.
package main

import (
	"context"
	"errors"

	"github.com/alecthomas/kong"

	"github.com/circleci-functions/toolkit/functionyaml"
	"github.com/circleci-functions/toolkit/sdk"
)

type cli struct {
	Install     installCmd     `cmd:"" default:"withargs" help:"Install Ruby and put ruby, gem and bundle on PATH. Runs when no command is given."`
	InstallGems installGemsCmd `cmd:"" help:"Install the project's gems with Bundler, restoring/saving the gem cache around the install."`
}

func main() {
	sdk.Run(run)
}

func run(ctx context.Context, client *sdk.Client) error {
	var flags cli
	kCtx := kong.Parse(&flags,
		kong.Name("setup-ruby"),
		kong.Description("Install Ruby, and optionally a project's gems, caching what each downloads.\n\nExamples:\n  circleci run github.com/circleci-functions/setup-ruby@<version> -- --ruby-version 3.4\n  circleci run github.com/circleci-functions/setup-ruby@<version> -- install-gems --app-dir ./api"),
		kong.UsageOnError(),
		functionyaml.Embed(),
	)
	kCtx.BindTo(ctx, (*context.Context)(nil))
	kCtx.Bind(client)
	return kCtx.Run(ctx)
}

type installCmd struct{}

func (c *installCmd) Run() error {
	return errors.New("installing Ruby is not implemented yet; run install-gems")
}
