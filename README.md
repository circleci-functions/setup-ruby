# setup-ruby

Installs your Ruby gems with Bundler and caches them, so later steps start from
a warm cache.

```yaml
steps:
  - run:
      name: Install gems
      command: circleci run setup-ruby@<version> -- --app-dir ./api
  - run: bundle exec rake
```

setup-ruby resolves the lock file in `--app-dir`, restores the gem cache,
installs the Bundler version the lock file was built with, installs the gems,
then saves the cache. Ruby and Bundler must already be on `PATH` — for example, a
`cimg/ruby` image.

## Choosing the lock file

By default setup-ruby installs the gems declared by the `Gemfile` in `--app-dir`.
Point it at a different directory or Gemfile when your project needs it:

```yaml
- run: circleci run setup-ruby@<version> -- --app-dir ./api --gemfile Gemfile
```

The lock file used for the cache key is resolved in this order:

1. `--override-cache-file`, when set
2. `Gemfile_next.lock`, when `DEPENDENCIES_NEXT` is set
3. a `.locked` sibling, for a `.rb` gemfile
4. `<gemfile>.lock`

The Bundler version comes from the lock file's `BUNDLED WITH` line; set
`--bundler-version` to install a specific one instead.

## Caching

Caching is on by default. setup-ruby restores the gem cache before installing and
saves it afterwards, keyed as `<key>-<arch>-<branch>-<checksum>`, where
`<checksum>` is a SHA-256 of the resolved lock file — so a changed lock file
refreshes the cache on its own.

Restore also tries a looser `<key>-<arch>-<branch>-` key, so a changed lock file
still warms from the last build; `--only-use-gemfile-checksum-cache-key` drops
that fallback. Drop `<arch>` or `<branch>` from the key with
`--include-arch-in-cache-key=false` or `--include-branch-in-cache-key=false`.

Turn caching off with `--cache=false`. Cache problems never fail the job, and a
directory with no lock file is installed without caching.

```yaml
- run: circleci run setup-ruby@<version> -- --key gems-v1 --clean-bundle
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--app-dir` | `.` | Directory containing the Gemfile |
| `--gemfile` | `Gemfile` | Gemfile name |
| `--path` | `./vendor/bundle` | Gem install path; the default enables Bundler deployment mode |
| `--key` | `gems-v1` | Cache key prefix |
| `--bundler-version` | (detected) | Bundler version to install; taken from the lock file when unset |
| `--clean-bundle` | `false` | Run `bundle clean --force` after install |
| `--override-cache-file` | (none) | Lock file to key the cache on instead of the detected one |
| `--cache` | `true` | Set `--cache=false` to disable caching |
| `--include-arch-in-cache-key` | `true` | Include the architecture in the cache key |
| `--include-branch-in-cache-key` | `true` | Include the branch in the cache key |
| `--only-use-gemfile-checksum-cache-key` | `false` | Restore only the exact checksum key, without the loose fallback |

## Platform support

Linux and macOS.
