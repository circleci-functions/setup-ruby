package rubydist

import (
	"bytes"
	"cmp"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"time"

	hc "github.com/circleci-functions/toolkit/httpcl"

	"github.com/circleci-functions/setup-ruby/rubyversion"
)

//go:embed downloads.json
var snapshot []byte

// Build is one downloadable archive: a Ruby build, or a toolchain.
type Build struct {
	// Version is the Ruby version, e.g. 3.4.1, or a toolchain's release.
	Version string `json:"version"`
	// Platform is the build's Platform, as its String method writes it.
	Platform string `json:"platform"`
	URL      string `json:"url"`
	SHA256   string `json:"sha256"`
	// Root is the directory in the archive that holds the install, or "" for
	// the archive's top level.
	Root string `json:"root"`
}

// Catalog lists the builds setup-ruby can install.
type Catalog struct {
	Rubies []Build `json:"rubies"`
	// Toolchains are the MSYS2 build tools for each Windows platform.
	Toolchains []Build `json:"toolchains"`
}

// Parse reads a catalog as Marshal writes it.
func Parse(data []byte) (Catalog, error) {
	var c Catalog
	if err := json.Unmarshal(data, &c); err != nil {
		return Catalog{}, fmt.Errorf("parse Ruby download list: %w", err)
	}
	return c, nil
}

// Snapshot returns the catalog built into setup-ruby.
func Snapshot() (Catalog, error) {
	return Parse(snapshot)
}

// Fetch downloads and parses the catalog at url.
func Fetch(ctx context.Context, url string) (Catalog, error) {
	data, err := get(ctx, url)
	if err != nil {
		return Catalog{}, err
	}
	return Parse(data)
}

// Versions returns the Ruby versions c holds builds of for p, oldest first.
func (c Catalog) Versions(p Platform) []string {
	var versions []string
	for _, b := range c.Rubies {
		if b.Platform == p.String() {
			versions = append(versions, b.Version)
		}
	}
	slices.SortFunc(versions, rubyversion.Compare)
	return slices.Compact(versions)
}

// Find returns the build of Ruby version for p.
func (c Catalog) Find(p Platform, version string) (Build, bool) {
	return find(c.Rubies, p, version)
}

// Toolchain returns the MSYS2 build tools for p.
func (c Catalog) Toolchain(p Platform) (Build, bool) {
	return find(c.Toolchains, p, "")
}

func find(builds []Build, p Platform, version string) (Build, bool) {
	for _, b := range builds {
		if b.Platform == p.String() && (version == "" || b.Version == version) {
			return b, true
		}
	}
	return Build{}, false
}

// Marshal writes c one build to a line, sorted, so a refreshed snapshot
// diffs by build.
func (c Catalog) Marshal() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("{\n")
	for i, list := range []struct {
		name   string
		builds []Build
	}{{"rubies", c.Rubies}, {"toolchains", c.Toolchains}} {
		builds := slices.Clone(list.builds)
		slices.SortFunc(builds, func(a, b Build) int {
			return cmp.Or(cmp.Compare(a.Platform, b.Platform), rubyversion.Compare(a.Version, b.Version))
		})
		fmt.Fprintf(&buf, "%q: [\n", list.name)
		for j, b := range builds {
			line, err := json.Marshal(b)
			if err != nil {
				return nil, err
			}
			buf.Write(line)
			if j < len(builds)-1 {
				buf.WriteString(",")
			}
			buf.WriteString("\n")
		}
		buf.WriteString("]")
		if i == 0 {
			buf.WriteString(",")
		}
		buf.WriteString("\n")
	}
	buf.WriteString("}\n")
	return buf.Bytes(), nil
}

func get(ctx context.Context, url string) ([]byte, error) {
	client := hc.New(ctx, hc.Config{Name: "ruby-downloads", Timeout: 2 * time.Minute})
	var data []byte
	err := client.Call(ctx, hc.NewRequest(http.MethodGet, "%s",
		hc.RouteParams(url),
		hc.BytesDecoder(&data),
		hc.Timeout(time.Minute),
	))
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", url, err)
	}
	return data, nil
}
