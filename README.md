# setup-ruby

Installs Ruby and puts `ruby`, `gem` and `bundle` on `PATH` for the rest of the
job. An optional `install-gems` command then installs your gems with Bundler,
caching them so later builds start from a warm cache.

```yaml
functions:
  setup-ruby: github.com/circleci-functions/setup-ruby@<version>

jobs:
  test:
    docker:
      - image: cimg/base:stable
    steps:
      - checkout
      - setup-ruby:
          with:
            ruby-version: "3.4"
      - setup-ruby/install-gems
      - run: bundle exec rake
```

`- setup-ruby:` runs the default command, which installs Ruby, and
`- setup-ruby/install-gems:` runs `install-gems`. Each flag in the tables
below is a `with:` key, written without its leading `--`.

Where the `functions:` block isn't enabled, such as on CircleCI Server, run the
same commands with `circleci run` and the function's full name:

```yaml
- run: circleci run github.com/circleci-functions/setup-ruby@<version> -- --ruby-version 3.4
- run: circleci run github.com/circleci-functions/setup-ruby@<version> -- install-gems
```

## Installing Ruby

With no command, setup-ruby downloads a prebuilt Ruby for the machine it runs
on, checks it against its SHA-256, and links `ruby`, `gem`, `bundle`,
`bundler`, `irb`, `rake`, `erb`, `rdoc` and `ri` into a directory already on
`PATH`. Later steps run them by name, in any shell, without sourcing anything.

The builds are relocatable, so they run from wherever setup-ruby unpacks them,
and need nothing from the system beyond its C library:

| Platform | Builds |
|----------|--------|
| Linux (glibc), amd64 and arm64 | [jdx/ruby](https://github.com/jdx/ruby), which need glibc 2.17 or newer |
| Linux (musl, such as Alpine), amd64 and arm64 | [rv-ruby](https://github.com/spinel-coop/rv-ruby) |
| macOS, arm64 | [jdx/ruby](https://github.com/jdx/ruby) |
| Windows, amd64 and arm64 | [RubyInstaller2](https://github.com/oneclick/rubyinstaller2) |

### Choosing a version

`--ruby-version` takes any of:

| Spec | Installs |
|------|----------|
| `3.4.1`, `3.5.0-preview1` | exactly that release |
| `3`, `3.4`, `3.4.x` | the newest matching stable release, or the newest prerelease while there is no stable one |
| `latest` | the newest stable release |

A leading `ruby-`, as in `ruby-3.4.1`, is ignored. Only CRuby is supported:
JRuby, TruffleRuby and development builds such as `head` are not.

Or read the version from a file with `--ruby-version-file`:

| File | Read from |
|------|-----------|
| `.ruby-version` | the first line that is not a comment |
| `.tool-versions` | the `ruby` line |
| `mise.toml` | `tools.ruby` |

```yaml
- setup-ruby:
    with:
      ruby-version-file: .tool-versions
```

With neither flag, setup-ruby reads `.ruby-version`, `.tool-versions` and then
`mise.toml` from `--working-directory`, the step's working directory unless you
set it. With none of them, it uses the Ruby already on `PATH` and installs
nothing. With no Ruby on `PATH` at all, it installs the newest stable release.
`--ruby-version` wins when both flags are set.

### Bundler and RubyGems

`--bundler` picks the Bundler to install:

| Value | Installs |
|-------|----------|
| `Gemfile.lock` (default) | the version in the `BUNDLED WITH` section of the lock file in `--working-directory`, or as `default` when there is none |
| `default` | nothing: the Bundler Ruby ships is used |
| `latest` | the newest Bundler |
| `none` | nothing |
| `2`, `2.6`, `2.6.9` | the newest matching release, unless it is already installed |

The lock file is found as Bundler finds it: from `$BUNDLE_LOCKFILE` or
`$BUNDLE_GEMFILE` when set, and otherwise beside `Gemfile` or `gems.rb`.

`--rubygems` updates RubyGems: `default` leaves the version Ruby ships,
`latest` runs `gem update --system`, and a version such as `3.6.9` updates to
it if it is newer than the one Ruby ships.

Both apply to the Ruby already on `PATH` too, when setup-ruby uses that one.
setup-ruby also writes a `~/.gemrc` that turns off documentation generation,
unless you have one.

### Gem executables

Where it can, setup-ruby has `gem install` put the executables of the gems it
installs beside `ruby` on `PATH`, so `gem install rubocop` makes `rubocop`
resolve by name in later steps. Where `$GEM_HOME` is set, they go to
`$GEM_HOME/bin` instead, and setup-ruby warns if that is not on `PATH`.

Where the directory `ruby` is linked into needs elevated permissions to write
to, they stay in the install's own `bin` directory, which is not on `PATH`.
setup-ruby warns when that happens; run them by their path there, or with
`bundle exec`. Under rbenv, gem install rehashes rbenv's shims instead, so the
executables resolve through them.

### rbenv

Where `ruby` is already on `PATH` through rbenv's shims, setup-ruby registers
the install with rbenv instead of linking it: it is added to
`$(rbenv root)/versions` under its version, made rbenv's global version, and
the shims rehashed. rbenv looks a `.ruby-version` up by name only, so the
install is also registered under the version it was asked for by, such as
`3.4`, unless rbenv already has a Ruby of that name. A `.ruby-version` file
in `--working-directory` or `$RBENV_VERSION` takes
precedence over rbenv's global version, so if either selects another Ruby,
setup-ruby fails rather than leave later steps running a Ruby other than the
one asked for.

### Where it links

setup-ruby links into the first of `~/.local/bin`, `~/bin` and
`/usr/local/bin` that is on `PATH` ahead of any `ruby` already there and that
it can write to. Failing that, it links into the directory of the `ruby`
already on `PATH`, replacing the `ruby`, `gem` and other executables there,
or, with no `ruby` on `PATH`, into the first directory on `PATH` it can write
to, then `/usr/local/bin`.

Where that directory needs root, as for a preinstalled `/usr/bin/ruby`,
setup-ruby links with `sudo -n`. Without passwordless sudo, it fails before
downloading anything. Put `~/.local/bin` on `PATH` ahead of that directory,
or name a directory with `--bin-dir`: setup-ruby links into exactly that one,
without searching `PATH` or registering with rbenv, so it has to be on `PATH`
for later steps.

A binary it replaces is kept beside the link as `<name>.pathlink-old`.

### Windows build tools

On Windows, setup-ruby also installs the MSYS2 build tools RubyInstaller uses
to compile gems with native extensions, into the install's `msys64`
directory, where `gem install` and `bundle install` find them by themselves.
They are a large download, cached like the install; skip them with
`--windows-toolchain none` when no gem needs compiling.

### Where the list of builds comes from

Each release of setup-ruby has the list of builds and their checksums built
in, so resolving a version needs no network. A partial version such as `3.4`
resolves to the newest release in that list; upgrade setup-ruby to pick up
newer ones. An exact version newer than the list is looked up where its builds
are published, with the checksum published beside them, on Linux and macOS.

### Caching the install

The unpacked install is saved to the job cache under a key made of the Ruby
version, platform and build, so later jobs restore it rather than downloading
it again, and a version rebuilt in a newer release of setup-ruby is
downloaded afresh. The Windows build tools are cached the same way. A cache problem never
fails the job. Turn it off with `--cache=false`.

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--ruby-version` | (see above) | Version spec to install |
| `--ruby-version-file` | (empty) | File to read the version spec from |
| `--working-directory` | `.` | Directory to look for version files and the lock file in |
| `--bundler` | `Gemfile.lock` | Bundler to install |
| `--rubygems` | `default` | RubyGems version to update to |
| `--windows-toolchain` | `default` | `default` installs the MSYS2 build tools on Windows, `none` skips them |
| `--bin-dir` | (empty) | Link `ruby`, `gem` and `bundle` into this directory instead of discovering one on `PATH` |
| `--cache` | `true` | Set `--cache=false` to disable caching the install |

## Installing gems

`install-gems` installs the gems declared by the `Gemfile` in `--app-dir` with
the `bundle` on `PATH`, restoring the gem cache before installing and saving it
afterwards.

```yaml
- setup-ruby/install-gems:
    with:
      app-dir: ./api
```

Run the install command first, or use an image with Ruby and Bundler on
`PATH`, such as `cimg/ruby`. Bundler switches to the version a lock file was
written with by itself; the install command's `--bundler` installs it ahead of
time. It replaces v0.1.0's `--bundler-version`; `install-gems` has no Bundler
flag.

### Choosing the lock file

By default `install-gems` installs the gems declared by the `Gemfile` in
`--app-dir`. Point it at a different directory or Gemfile when your project
needs it:

```yaml
- setup-ruby/install-gems:
    with:
      app-dir: ./api
      gemfile: Gemfile
```

The lock file used for the cache key is resolved in this order:

1. `--override-cache-file`, when set
2. `Gemfile_next.lock`, when `DEPENDENCIES_NEXT` is set
3. a `.locked` sibling, for a `.rb` gemfile
4. `<gemfile>.lock`

### Caching gems

Caching is on by default. `install-gems` restores the gem cache before
installing and saves it afterwards, keyed as
`<key>-<arch>-<branch>-<checksum>`, where `<checksum>` is a SHA-256 of the
resolved lock file — so a changed lock file refreshes the cache on its own.
It saves after every install, even one restored under that exact key: cache
keys are written once, so that save is skipped rather than overwriting
anything.

Restore also tries a looser `<key>-<arch>-<branch>-` key, so a changed lock
file still warms from the last build; `--only-use-gemfile-checksum-cache-key`
drops that fallback. Drop `<arch>` or `<branch>` from the key with
`--include-arch-in-cache-key=false` or `--include-branch-in-cache-key=false`.

Turn caching off with `--cache=false`. Cache problems never fail the job, and
a directory with no lock file is installed without caching.

```yaml
- setup-ruby/install-gems:
    with:
      key: gems-v1
      clean-bundle: true
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--app-dir` | `.` | Directory containing the Gemfile |
| `--gemfile` | `Gemfile` | Gemfile name |
| `--path` | `./vendor/bundle` | Gem install path; the default enables Bundler deployment mode |
| `--key` | `gems-v1` | Cache key prefix |
| `--clean-bundle` | `false` | Run `bundle clean --force` after install |
| `--override-cache-file` | (none) | Lock file to key the cache on instead of the detected one |
| `--cache` | `true` | Set `--cache=false` to disable caching |
| `--include-arch-in-cache-key` | `true` | Include the architecture in the cache key |
| `--include-branch-in-cache-key` | `true` | Include the branch in the cache key |
| `--only-use-gemfile-checksum-cache-key` | `false` | Restore only the exact checksum key, without the loose fallback |

## Platform support

Linux on amd64 and arm64, with glibc 2.17 or newer or with musl; macOS on
arm64; and Windows on amd64 and arm64. A version spec only matches releases
with a build for the machine it runs on.

- **Ruby versions** go back to 3.2 everywhere but Linux with musl, where they
  go back to 3.3.
- **macOS on Intel** has no builds; on one, an install fails, though
  `install-gems` works with a Ruby that is already there.
- **Windows** gets `ruby`, `gem`, `bundle` and the other executables as shims
  that work from cmd, PowerShell and Git Bash. The `.bat` wrappers `gem
  install` and `bundle install` write run the installed `ruby.exe` by its
  path.
