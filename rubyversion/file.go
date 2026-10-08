package rubyversion

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// DefaultFiles are the version files read, in order, when no version or
// version file is given.
var DefaultFiles = []string{".ruby-version", ".tool-versions", "mise.toml"}

// ErrNoVersion is returned by FromFile for a file that names no version.
var ErrNoVersion = errors.New("no Ruby version found")

// FromFile reads a version spec from path, by the file's name:
//
//   - .tool-versions: the ruby line
//   - a .toml file, such as mise.toml: tools.ruby
//   - anything else, such as .ruby-version: the first line that is not a
//     comment
func FromFile(path string) (string, error) {
	data, err := os.ReadFile(path) //nolint:gosec // the caller names the file to read
	if err != nil {
		return "", err
	}

	data = bytes.TrimPrefix(data, []byte("\ufeff"))
	var spec string
	switch {
	case filepath.Base(path) == ".tool-versions":
		spec = fromToolVersions(string(data))
	case filepath.Ext(path) == ".toml":
		spec, err = fromMise(data)
	default:
		spec = firstLine(string(data))
	}
	if err != nil {
		return "", fmt.Errorf("%s: %w", path, err)
	}
	if spec == "" {
		return "", fmt.Errorf("%s: %w", path, ErrNoVersion)
	}
	return spec, nil
}

// Default reads the spec from the first of DefaultFiles in dir that names a
// version, returning the file it read, or an empty spec and source when none
// does.
func Default(dir string) (spec, source string, err error) {
	for _, name := range DefaultFiles {
		path := filepath.Join(dir, name)
		spec, err = FromFile(path)
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, ErrNoVersion) {
			continue
		}
		if err != nil {
			return "", "", err
		}
		return spec, path, nil
	}
	return "", "", nil
}

func firstLine(data string) string {
	for line := range strings.Lines(data) {
		if fields := strings.Fields(line); len(fields) > 0 && !strings.HasPrefix(fields[0], "#") {
			return fields[0]
		}
	}
	return ""
}

func fromToolVersions(data string) string {
	for line := range strings.Lines(data) {
		if fields := strings.Fields(line); len(fields) > 1 && fields[0] == "ruby" {
			return fields[1]
		}
	}
	return ""
}

// fromMise reads tools.ruby, which mise allows as a version, a list of
// versions whose first is the default, or a table with a version key.
func fromMise(data []byte) (string, error) {
	var doc struct {
		Tools map[string]any `toml:"tools"`
	}
	if err := toml.Unmarshal(data, &doc); err != nil {
		return "", err
	}
	switch v := doc.Tools["ruby"].(type) {
	case string:
		return v, nil
	case []any:
		if len(v) > 0 {
			s, _ := v[0].(string)
			return s, nil
		}
	case map[string]any:
		s, _ := v["version"].(string)
		return s, nil
	}
	return "", nil
}
