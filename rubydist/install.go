package rubydist

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	hc "github.com/circleci-functions/toolkit/httpcl"
)

// marker is the file Install leaves in an install, naming the build it holds.
const marker = ".setup-ruby-build"

// Installed reports whether dest holds b as Install left it, so that neither
// what an interrupted run or a failed cache restore left at dest, nor another
// build of the same version, passes for it.
func Installed(dest string, b Build) bool {
	data, err := os.ReadFile(filepath.Join(dest, marker)) //nolint:gosec // dest is setup-ruby's own install
	return err == nil && strings.TrimSpace(string(data)) == strings.ToLower(b.SHA256)
}

// Install downloads b, checks it against its SHA-256, and unpacks it so that
// dest is the directory the archive holds at b.Root, marked as holding b.
// Anything already at dest, such as an incomplete install, is replaced.
func Install(ctx context.Context, b Build, dest string) error {
	name := path.Base(b.URL)
	parent := filepath.Dir(dest)
	if err := os.MkdirAll(parent, 0750); err != nil {
		return err
	}
	archive, err := os.CreateTemp(parent, name+".*.download")
	if err != nil {
		return err
	}
	defer func() {
		_ = archive.Close()
		_ = os.Remove(archive.Name())
	}()

	if err := download(ctx, b.URL, archive); err != nil {
		return err
	}
	if err := verify(archive, b.SHA256); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}

	staging, err := os.MkdirTemp(parent, filepath.Base(dest)+".*.extract")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(staging) }()

	extract := extractTarGz
	if strings.HasSuffix(name, ".7z") {
		extract = extract7z
	}
	if err := extract(archive.Name(), staging); err != nil {
		return fmt.Errorf("extract %s: %w", name, err)
	}
	root := filepath.Join(staging, filepath.FromSlash(b.Root))
	if _, err := os.Stat(root); err != nil {
		return fmt.Errorf("%s has no %s directory", name, b.Root)
	}
	if err := os.WriteFile(filepath.Join(root, marker), []byte(strings.ToLower(b.SHA256)+"\n"), 0644); err != nil { //nolint:gosec // not secret
		return err
	}
	if err := os.RemoveAll(dest); err != nil {
		return err
	}
	return os.Rename(root, dest)
}

// download writes the file at url to f, starting over on each retry.
func download(ctx context.Context, url string, f *os.File) error {
	client := hc.New(ctx, hc.Config{Name: "ruby-builds", Timeout: 15 * time.Minute})
	err := client.Call(ctx, hc.NewRequest(http.MethodGet, "%s",
		hc.RouteParams(url),
		hc.SuccessDecoder(func(r io.Reader) error {
			if _, err := f.Seek(0, io.SeekStart); err != nil {
				return err
			}
			if err := f.Truncate(0); err != nil {
				return err
			}
			_, err := io.Copy(f, r)
			return err
		}),
		hc.Timeout(10*time.Minute),
	))
	if err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	return nil
}

func verify(f *os.File, want string) error {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != strings.ToLower(want) {
		return fmt.Errorf("checksum mismatch: got sha256 %s, want %s", got, want)
	}
	return nil
}
