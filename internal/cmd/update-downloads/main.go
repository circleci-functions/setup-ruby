// Command update-downloads refreshes the catalog of Ruby builds embedded in
// setup-ruby from the GitHub releases the builds are published in. It honours
// $GITHUB_TOKEN, which lifts the API's rate limit for anonymous callers.
//
//	go run ./internal/cmd/update-downloads [-o rubydist/downloads.json]
package main

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/circleci-functions/setup-ruby/rubydist"
	"github.com/circleci-functions/setup-ruby/rubyversion"
)

// oldest is the oldest Ruby the catalog lists, the first jdx/ruby builds.
const oldest = "3.2.0"

func main() {
	out := flag.String("o", "rubydist/downloads.json", "file to write the catalog to")
	flag.Parse()

	if err := run(context.Background(), *out); err != nil {
		fmt.Fprintln(os.Stderr, "update-downloads:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, out string) error {
	gh := github{
		token:    os.Getenv("GITHUB_TOKEN"),
		client:   &http.Client{Timeout: time.Minute},
		download: &http.Client{Timeout: 10 * time.Minute},
	}
	var c rubydist.Catalog
	for _, add := range []func(context.Context, github, *rubydist.Catalog) error{addJDX, addRV, addRubyInstaller, addMSYS2} {
		if err := add(ctx, gh, &c); err != nil {
			return err
		}
	}
	data, err := c.Marshal()
	if err != nil {
		return err
	}
	if err := os.WriteFile(out, data, 0644); err != nil { //nolint:gosec // a checked-in source file
		return err
	}
	fmt.Printf("wrote %d Ruby builds and %d toolchains to %s\n", len(c.Rubies), len(c.Toolchains), out)
	return nil
}

var (
	jdxTag           = regexp.MustCompile(`^(\d+\.\d+\.\d+(?:-[a-z]+\d*)?)(?:-(\d+))?$`)
	rvAsset          = regexp.MustCompile(`^ruby-(\d+\.\d+\.\d+(?:-[a-z]+\d*)?)\.[a-z0-9_]+_linux_musl\.tar\.gz$`)
	rubyInstallerTag = regexp.MustCompile(`^RubyInstaller-(\d+\.\d+\.\d+)-(\d+)$`)
)

var (
	jdxPlatforms           = platforms("linux-x86_64-gnu", "linux-aarch64-gnu", "darwin-aarch64")
	rvPlatforms            = platforms("linux-x86_64-musl", "linux-aarch64-musl")
	rubyInstallerPlatforms = platforms("windows-x86_64", "windows-aarch64")
)

// addJDX adds jdx/ruby's builds, from each version's newest revision. A
// version's releases are tagged with the version, and its rebuilds with the
// version and a revision number; the archives in each are named for the
// version alone.
func addJDX(ctx context.Context, gh github, c *rubydist.Catalog) error {
	releases, err := gh.releases(ctx, rubydist.JDXRepo)
	if err != nil {
		return err
	}
	newest := newestRevisions(releases, func(tag string) (string, int, bool) {
		m := jdxTag.FindStringSubmatch(tag)
		if m == nil {
			return "", 0, false
		}
		rev, _ := strconv.Atoi(cmp.Or(m[2], "0"))
		return m[1], rev, true
	})
	for version, rel := range newest {
		for _, p := range jdxPlatforms {
			asset, _ := rubydist.JDXAsset(version, p)
			if err := addBuild(ctx, gh, &c.Rubies, rel, asset, version, p, rubydist.JDXRoot(version)); err != nil {
				return err
			}
		}
	}
	return nil
}

// addRV adds rv-ruby's musl builds from its latest release, which holds
// every version it builds.
func addRV(ctx context.Context, gh github, c *rubydist.Catalog) error {
	rel, err := gh.latest(ctx, rubydist.RVRepo)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, a := range rel.Assets {
		m := rvAsset.FindStringSubmatch(a.Name)
		if m == nil || rubyversion.Compare(m[1], oldest) < 0 || seen[m[1]] {
			continue
		}
		seen[m[1]] = true
		for _, p := range rvPlatforms {
			asset, _ := rubydist.RVAsset(m[1], p)
			if err := addBuild(ctx, gh, &c.Rubies, rel, asset, m[1], p, rubydist.RVRoot(m[1])); err != nil {
				return err
			}
		}
	}
	return nil
}

// addRubyInstaller adds RubyInstaller2's builds, from each version's newest
// packaging revision.
func addRubyInstaller(ctx context.Context, gh github, c *rubydist.Catalog) error {
	releases, err := gh.releases(ctx, rubydist.RubyInstallerRepo)
	if err != nil {
		return err
	}
	newest := newestRevisions(releases, func(tag string) (string, int, bool) {
		m := rubyInstallerTag.FindStringSubmatch(tag)
		if m == nil {
			return "", 0, false
		}
		rev, _ := strconv.Atoi(m[2])
		return m[1], rev, true
	})
	for version, rel := range newest {
		_, revision, _ := strings.Cut(strings.TrimPrefix(rel.TagName, "RubyInstaller-"+version), "-")
		for _, p := range rubyInstallerPlatforms {
			asset, _ := rubydist.RubyInstallerAsset(version, revision, p)
			if err := addBuild(ctx, gh, &c.Rubies, rel, asset, version, p, rubydist.RubyInstallerRoot(asset)); err != nil {
				return err
			}
		}
	}
	return nil
}

// addMSYS2 adds the MSYS2 build tools from ruby/setup-msys2-gcc's latest
// release. Each release is tagged with the time it was built and kept, so
// the catalog pins one.
func addMSYS2(ctx context.Context, gh github, c *rubydist.Catalog) error {
	rel, err := gh.latest(ctx, rubydist.MSYS2Repo)
	if err != nil {
		return err
	}
	for _, p := range rubyInstallerPlatforms {
		asset, _ := rubydist.MSYS2Asset(p)
		if err := addBuild(ctx, gh, &c.Toolchains, rel, asset, rel.TagName, p, ""); err != nil {
			return err
		}
	}
	return nil
}

// newestRevisions returns the release with the highest revision of each Ruby
// version from oldest on, as parse reads them from release tags.
func newestRevisions(releases []release, parse func(tag string) (version string, revision int, ok bool)) map[string]release {
	newest := map[string]release{}
	revisions := map[string]int{}
	for _, rel := range releases {
		version, rev, ok := parse(rel.TagName)
		if !ok || rel.Draft || rubyversion.Compare(version, oldest) < 0 {
			continue
		}
		if prev, seen := revisions[version]; seen && prev >= rev {
			continue
		}
		newest[version], revisions[version] = rel, rev
	}
	return newest
}

// addBuild appends the build rel holds as asset to builds, if it holds one.
// GitHub only records digests for assets uploaded since mid-2025, so an older
// asset is downloaded and hashed instead.
func addBuild(ctx context.Context, gh github, builds *[]rubydist.Build, rel release, asset, version string, p rubydist.Platform, root string) error {
	for _, a := range rel.Assets {
		if a.Name != asset {
			continue
		}
		sum, ok := strings.CutPrefix(a.Digest, "sha256:")
		if !ok {
			var err error
			if sum, err = gh.hash(ctx, a.URL); err != nil {
				return err
			}
		}
		*builds = append(*builds, rubydist.Build{Version: version, Platform: p.String(), URL: a.URL, SHA256: sum, Root: root})
		return nil
	}
	return nil
}

func platforms(names ...string) []rubydist.Platform {
	out := make([]rubydist.Platform, 0, len(names))
	for _, name := range names {
		parts := strings.Split(name, "-")
		p := rubydist.Platform{OS: parts[0], Arch: parts[1], Libc: "none"}
		if len(parts) == 3 {
			p.Libc = parts[2]
		}
		out = append(out, p)
	}
	return out
}

type release struct {
	TagName string  `json:"tag_name"`
	Draft   bool    `json:"draft"`
	Assets  []asset `json:"assets"`
}

type asset struct {
	Name   string `json:"name"`
	URL    string `json:"browser_download_url"`
	Digest string `json:"digest"`
}

type github struct {
	token    string
	client   *http.Client
	download *http.Client
}

func (g github) releases(ctx context.Context, repo string) ([]release, error) {
	var all []release
	for page := 1; ; page++ {
		var batch []release
		if err := g.get(ctx, fmt.Sprintf("repos/%s/releases?per_page=100&page=%d", repo, page), &batch); err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			return all, nil
		}
		all = append(all, batch...)
	}
}

func (g github) latest(ctx context.Context, repo string) (release, error) {
	var rel release
	err := g.get(ctx, fmt.Sprintf("repos/%s/releases/latest", repo), &rel)
	return rel, err
}

// hash downloads the file at url and returns its SHA-256.
func (g github) hash(ctx context.Context, url string) (string, error) {
	fmt.Fprintf(os.Stderr, "update-downloads: no digest for %s, hashing it\n", url)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := g.download.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	h := sha256.New()
	if _, err := io.Copy(h, resp.Body); err != nil {
		return "", fmt.Errorf("GET %s: %w", url, err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func (g github) get(ctx context.Context, path string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/"+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if g.token != "" {
		req.Header.Set("Authorization", "Bearer "+g.token)
	}
	resp, err := g.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", path, resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}
