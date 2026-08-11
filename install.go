package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/circleci-functions/setup-ruby/ruby"
)

type installCmd struct {
	Version     string `help:"Ruby version to install, e.g. 3.3.0." required:""`
	OpenSSLPath string `name:"openssl-path" help:"Directory of an OpenSSL install to build against (overrides --with-openssl-dir)."`
}

func (c *installCmd) Run(ctx context.Context) error {
	if rubyAlreadyInstalled(ctx, c.Version) {
		fmt.Fprintf(os.Stderr, "setup-ruby: ruby %s already installed\n", c.Version)
		return nil
	}
	if runtime.GOOS == "darwin" {
		return installWithRbenv(ctx, c.Version)
	}
	return installWithRVM(ctx, c.Version, c.OpenSSLPath)
}

func rubyAlreadyInstalled(ctx context.Context, version string) bool {
	out, err := ruby.Output(ctx, "", "ruby", "--version")
	return err == nil && strings.Contains(out, version)
}

func installWithRbenv(ctx context.Context, version string) error {
	if err := ruby.Run(ctx, "", "rbenv", "install", version); err != nil {
		return fmt.Errorf("rbenv install %s: %w", version, err)
	}
	if err := ruby.Run(ctx, "", "rbenv", "global", version); err != nil {
		return fmt.Errorf("rbenv global %s: %w", version, err)
	}
	return nil
}

func installWithRVM(ctx context.Context, version, opensslPath string) error {
	if err := ensureRVM(ctx); err != nil {
		return err
	}
	return rvmInstallRuby(ctx, version, opensslPath)
}

func ensureRVM(ctx context.Context) error {
	if ruby.Succeeds(ctx, "", "rvm", "--version") {
		return ruby.Run(ctx, "", "rvm", "get", "head")
	}
	if err := importRVMKeys(ctx); err != nil {
		return err
	}
	if err := ruby.RunShell(ctx, "", "curl -sSL https://get.rvm.io | bash -s stable"); err != nil {
		return fmt.Errorf("install rvm: %w", err)
	}
	return exportRVMEnv()
}

func importRVMKeys(ctx context.Context) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home: %w", err)
	}
	gnupg := filepath.Join(home, ".gnupg")
	if err := os.MkdirAll(gnupg, 0o700); err != nil {
		return fmt.Errorf("create %s: %w", gnupg, err)
	}
	if err := os.WriteFile(filepath.Join(gnupg, "dirmngr.conf"), []byte("disable-ipv6\n"), 0o600); err != nil {
		return fmt.Errorf("write dirmngr.conf: %w", err)
	}

	for _, server := range ruby.GPGKeyservers {
		fmt.Fprintf(os.Stderr, "setup-ruby: fetching RVM keys from %s\n", server)
		if ruby.Succeeds(ctx, "", "gpg", "--keyserver", server, "--keyserver-options", "timeout=10",
			"--recv-keys", ruby.RVMKeyMpapis, ruby.RVMKeyPkuczynski) {
			return trustRVMKeys(ctx)
		}
	}

	fmt.Fprintln(os.Stderr, "setup-ruby: keyservers unavailable, importing keys from rvm.io")
	if err := ruby.RunShell(ctx, "",
		"curl -sSL https://rvm.io/mpapis.asc | gpg --import - && curl -sSL https://rvm.io/pkuczynski.asc | gpg --import -"); err != nil {
		return fmt.Errorf("import rvm keys: %w", err)
	}
	return trustRVMKeys(ctx)
}

func trustRVMKeys(ctx context.Context) error {
	trust := fmt.Sprintf("%s:6:\n%s:6:\n", ruby.RVMKeyMpapis, ruby.RVMKeyPkuczynski)
	if err := ruby.RunStdin(ctx, "", trust, "gpg", "--import-ownertrust"); err != nil {
		return fmt.Errorf("trust rvm keys: %w", err)
	}
	return nil
}

func rvmInstallRuby(ctx context.Context, version, opensslPath string) error {
	var withOpenSSL string
	if runtime.GOOS != "darwin" && ruby.NeedsOldOpenSSL(version) {
		var err error
		if withOpenSSL, err = resolveOpenSSL(ctx, opensslPath); err != nil {
			return err
		}
	} else {
		if err := ruby.Run(ctx, "", "rvm", "autolibs", "enable"); err != nil {
			return fmt.Errorf("rvm autolibs enable: %w", err)
		}
	}

	if err := ruby.Run(ctx, "", "rvm", "get", "master"); err != nil {
		return fmt.Errorf("rvm get master: %w", err)
	}
	installArgs := []string{"install", version}
	if withOpenSSL != "" {
		installArgs = append(installArgs, withOpenSSL)
	}
	if err := ruby.Run(ctx, "", "rvm", installArgs...); err != nil {
		return fmt.Errorf("rvm install %s: %w", version, err)
	}
	if err := ruby.Run(ctx, "", "rvm", "use", version); err != nil {
		return fmt.Errorf("rvm use %s: %w", version, err)
	}

	return ruby.RunShell(ctx, "", fmt.Sprintf(
		`RUBY_PATH="$(rvm %s 1>/dev/null 2>/dev/null && rvm env --path)"; printf '%%s\n' "source $RUBY_PATH" >> "$BASH_ENV"`,
		version))
}

func resolveOpenSSL(ctx context.Context, opensslPath string) (string, error) {
	if opensslPath != "" {
		fmt.Fprintf(os.Stderr, "setup-ruby: using OpenSSL at %s\n", opensslPath)
		return "--with-openssl-dir=" + opensslPath, nil
	}
	if out, err := ruby.Output(ctx, "", "openssl", "version"); err == nil && supportedOpenSSL(out) {
		return "", nil
	}
	fmt.Fprintln(os.Stderr, "setup-ruby: no supported OpenSSL found, installing the rvm openssl package")
	if err := ruby.Run(ctx, "", "rvm", "pkg", "install", "openssl"); err != nil {
		return "", fmt.Errorf("rvm pkg install openssl: %w", err)
	}
	return "--with-openssl-dir=" + filepath.Join(ruby.RVMHome(), "usr"), nil
}

func supportedOpenSSL(version string) bool {
	fields := strings.Fields(version)
	if len(fields) < 2 {
		return false
	}
	parts := strings.SplitN(fields[1], ".", 2)
	return atoiSafe(parts[0]) == 1
}

func atoiSafe(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			break
		}
		n = n*10 + int(r-'0')
	}
	return n
}

func exportRVMEnv() error {
	bashEnv := os.Getenv("BASH_ENV")
	if bashEnv == "" {
		return fmt.Errorf("BASH_ENV must be set to configure RVM")
	}
	home := ruby.RVMHome()
	lines := fmt.Sprintf("export RVM_HOME=%s\nexport PATH=$PATH:%s/bin\nsource %s/scripts/rvm\n", home, home, home)

	f, err := os.OpenFile(bashEnv, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600) //nolint:gosec // BASH_ENV path is provided by CircleCI
	if err != nil {
		return fmt.Errorf("open BASH_ENV: %w", err)
	}
	defer f.Close()
	if _, err := f.WriteString(lines); err != nil {
		return fmt.Errorf("write BASH_ENV: %w", err)
	}
	return nil
}
