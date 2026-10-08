package rubydist

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/bodgit/sevenzip"

	"github.com/circleci-functions/toolkit/archive"
)

func extractTarGz(src, dest string) error {
	f, err := os.Open(src) //nolint:gosec // src is the archive this package just downloaded
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if err := extractTarEntry(tr, hdr, dest); err != nil {
			return err
		}
	}
}

func extractTarEntry(tr *tar.Reader, hdr *tar.Header, dest string) error {
	target, err := within(dest, hdr.Name)
	if err != nil {
		return err
	}
	mode := hdr.FileInfo().Mode().Perm()

	switch hdr.Typeflag {
	case tar.TypeDir:
		return os.MkdirAll(target, mode|0700)
	case tar.TypeReg:
		return writeFile(target, tr, mode)
	case tar.TypeSymlink:
		//nolint:gosec // this is the traversal check for the link's target
		if _, err := within(dest, filepath.Join(filepath.Dir(hdr.Name), hdr.Linkname)); err != nil || filepath.IsAbs(hdr.Linkname) {
			return fmt.Errorf("symlink %s points outside the archive", hdr.Name)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0750); err != nil {
			return err
		}
		return os.Symlink(hdr.Linkname, target)
	case tar.TypeLink:
		source, err := within(dest, hdr.Linkname)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0750); err != nil {
			return err
		}
		return os.Link(source, target)
	}
	return nil
}

// extract7z unpacks a 7-Zip archive, as RubyInstaller2 and MSYS2 publish
// theirs. Files are read in archive order, which lets a solid archive be
// decompressed in one pass rather than once per file.
func extract7z(src, dest string) error {
	r, err := sevenzip.OpenReader(src)
	if err != nil {
		return err
	}
	defer func() { _ = r.Close() }()

	for _, f := range r.File {
		target, err := within(dest, f.Name)
		if err != nil {
			return err
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0750); err != nil {
				return err
			}
			continue
		}
		if err := extract7zFile(f, target); err != nil {
			return err
		}
	}
	return nil
}

func extract7zFile(f *sevenzip.File, target string) error {
	rc, err := f.Open()
	if err != nil {
		return fmt.Errorf("%s: %w", f.Name, err)
	}
	defer func() { _ = rc.Close() }()
	return writeFile(target, rc, f.Mode().Perm())
}

func writeFile(target string, r io.Reader, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(target), 0750); err != nil {
		return err
	}
	return archive.WriteFile(target, r, mode|0600) //nolint:gosec // decompression size is bounded by the checksummed archive
}

// within joins name onto dest, rejecting any name that would land outside it.
func within(dest, name string) (string, error) {
	target := filepath.Join(dest, filepath.FromSlash(name))
	if target != filepath.Clean(dest) && !strings.HasPrefix(target, filepath.Clean(dest)+string(os.PathSeparator)) {
		return "", fmt.Errorf("archive entry %q escapes the destination", name)
	}
	return target, nil
}
