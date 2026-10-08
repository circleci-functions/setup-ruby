// Command setup-ruby installs Ruby and, through its install-gems subcommand, a
// project's gems with Bundler, caching what each downloads.
//
// Supported platforms: linux, darwin, and windows.
package main

import (
	"context"

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
		kong.Description("Install Ruby, and optionally a project's gems, caching what each downloads."),
		kong.UsageOnError(),
		functionyaml.Embed(),
	)
	kCtx.BindTo(ctx, (*context.Context)(nil))
	kCtx.Bind(client)
	return kCtx.Run(ctx)
}
