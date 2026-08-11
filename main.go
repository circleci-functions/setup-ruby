// Command setup-ruby installs Ruby gems with Bundler (with optional caching),
// runs RSpec and RuboCop, and installs Ruby itself via a version manager. Each
// orb command maps to one subcommand.
package main

import (
	"context"

	"github.com/alecthomas/kong"

	"github.com/circleci-functions/toolkit/functionyaml"
	"github.com/circleci-functions/toolkit/sdk"
)

type cli struct{}

func main() {
	sdk.Run(run)
}

func run(ctx context.Context, client *sdk.Client) error {
	var flags cli
	kCtx := kong.Parse(&flags,
		kong.Name("setup-ruby"),
		kong.Description("Install Ruby gems, run tests and lint, or install Ruby.\n\nExample:\n  circleci run github.com/circleci-functions/setup-ruby@<version> -- install-deps --app-dir ./api"),
		kong.UsageOnError(),
		functionyaml.Embed(),
	)
	kCtx.BindTo(ctx, (*context.Context)(nil))
	kCtx.Bind(client)
	return kCtx.Run(ctx)
}
