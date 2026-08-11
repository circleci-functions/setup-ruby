# setup-ruby

A CircleCI function to install Ruby gems with Bundler (with caching), run RSpec, run
RuboCop, and install Ruby itself.

Invoke it from a `run:` step. Use the full path and put `--` before the subcommand so the
CLI passes the remaining flags to the function:

```yaml
- run:
    command: circleci run github.com/circleci-functions/setup-ruby@<version> -- install-deps
```

Ruby and Bundler must already be on `PATH` (for example, a `cimg/ruby` image).

## install-deps

Installs gems with Bundler and, by default, caches them. It resolves the lock file, restores
the cache, installs the matching Bundler and the gems, then saves the cache.

| Flag | Default | Description |
|---|---|---|
| `--app-dir` | `.` | Directory containing the Gemfile. |
| `--gemfile` | `Gemfile` | Gemfile name. |
| `--path` | `./vendor/bundle` | Gem install path; the default enables Bundler deployment mode. |
| `--key` | `gems-v1` | Cache key prefix. |
| `--bundler-version` | detected | Bundler version to install; taken from the lock file when unset. |
| `--clean-bundle` | `false` | Run `bundle clean --force` after install. |
| `--override-cache-file` | none | Lock file to key the cache on instead of the detected one. |
| `--with-cache` / `--no-with-cache` | enabled | Restore and save the gem cache. |
| `--include-arch-in-cache-key` / `--no-…` | enabled | Include the architecture in the cache key. |
| `--include-branch-in-cache-key` / `--no-…` | enabled | Include the branch in the cache key. |
| `--only-use-gemfile-checksum-cache-key` | `false` | Restore only the exact checksum key, without the loose fallback. |

Lock file resolution, in order: `--override-cache-file`, then `Gemfile_next.lock` when
`DEPENDENCIES_NEXT` is set, then a `.locked` sibling for a `.rb` gemfile, then
`<gemfile>.lock`.

**Caching.** The cache key is `<key>-<arch>-<branch>-<checksum>`, where `<checksum>` is a
SHA-256 of the resolved lock file, `<arch>` is the OS/architecture, and `<branch>` is
`$CIRCLE_BRANCH`. Restore also tries a looser `<key>-<arch>-<branch>-` key so a changed lock
file still warms from the last build; `--only-use-gemfile-checksum-cache-key` drops that
fallback. Drop `<arch>` or `<branch>` with the matching `--no-…` flag, and turn caching off
entirely with `--no-with-cache`. Cache errors are logged and never fail the build.

```yaml
- run:
    name: Install deps
    command: >-
      circleci run github.com/circleci-functions/setup-ruby@<version> --
      install-deps --key gems-v1 --clean-bundle
```

## rspec-test

Runs RSpec with CircleCI test splitting and writes a JUnit report to `<out-path>/results.xml`.
With `--rerun-fail` (the default) it runs tests through `circleci tests run`, enabling the
"rerun failed tests only" feature; `--no-rerun-fail` splits by timings and runs `rspec`
directly. Your Gemfile needs `gem 'rspec_junit_formatter'`, and the `circleci` CLI must be
available in the job.

| Flag | Default | Description |
|---|---|---|
| `--app-dir` | `.` | Directory containing the Gemfile. |
| `--include` | `spec/**/*_spec.rb` | Glob(s) for test files; comma-separate multiple inside a single brace glob. |
| `--order` | none | RSpec `--order` value. |
| `--tag` | none | Space-separated RSpec tags to run. |
| `--out-path` | `/tmp/test-results/rspec` | Directory for `results.xml`. |
| `--rerun-fail` / `--no-rerun-fail` | enabled | Run via `circleci tests run` so failed tests can be rerun. |

The function writes the report but does not store it. Add the native steps yourself:

```yaml
- run:
    command: circleci run github.com/circleci-functions/setup-ruby@<version> -- rspec-test
- store_test_results:
    path: /tmp/test-results/rspec
- store_artifacts:
    path: /tmp/test-results/rspec
    destination: test-results
```

## rubocop-check

Runs RuboCop over `--check-path` and writes a report to `<out-path>/<out-file-name>`. Your
Gemfile needs `gem 'rubocop'`.

| Flag | Default | Description |
|---|---|---|
| `--app-dir` | `.` | Directory containing the Gemfile. |
| `--check-path` | `.` | Path RuboCop inspects. |
| `--format` | `progress` | RuboCop formatter. |
| `--out-file-name` | `check-results.xml` | Output file name. |
| `--out-path` | `/tmp/rubocop-results` | Directory for the output file. |
| `--parallel` / `--no-parallel` | enabled | Inspect files in parallel (`--no-parallel` is used only on RuboCop ≥ 1.13). |

```yaml
- run:
    command: circleci run github.com/circleci-functions/setup-ruby@<version> -- rubocop-check --format progress
```

## install

Installs Ruby with a version manager: RVM on Linux, rbenv on macOS. It skips the work when
the requested version is already active. On Linux it installs RVM (importing its signing
keys) when absent, and handles OpenSSL 1.1 for Ruby 3.0 and earlier. The Ruby environment is
exported to `$BASH_ENV` so later steps pick it up.

| Flag | Default | Description |
|---|---|---|
| `--version` | required | Ruby version to install, e.g. `3.3.0`. |
| `--openssl-path` | none | Directory of an OpenSSL install to build against (Linux, older Ruby). |

`--version` is a literal version. For a dynamic value, let your own shell expand it in the
`run:` step, for example `-- install --version "$(cat .ruby-version)"`.

Prefer a Ruby-preinstalled image (such as `cimg/ruby`) where you can; installing Ruby at run
time is slower and less deterministic.

```yaml
- run:
    command: circleci run github.com/circleci-functions/setup-ruby@<version> -- install --version 3.3.0
```

## Platform support

Linux and macOS. `install-deps` also handles a Windows Git-Bash environment by using
`%HOMEDRIVE%` for its temporary directory.

## Parameters not carried over from the Ruby orb

Kept here for reference when migrating from `circleci/ruby`:

| Command | Parameter | Why it is not a flag |
|---|---|---|
| install-deps | `no_output_timeout` | A property of the `run:` step; set it on the step that invokes the function. |
| install-deps | `pre-install-steps` | A `steps` value cannot be spliced into a single process; run your own steps around the invocation. |
| rspec-test | `label` | The `run:` step's name; set it on your step. |
| rspec-test | `no_output_timeout` | As above. |
| rspec-test | `store_test_results`, `store_artifacts` | Native steps with no function equivalent; add them yourself (shown above). |
