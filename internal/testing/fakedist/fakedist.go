// Package fakedist serves a stand-in for setup-ruby's catalog of Ruby builds,
// the archives it lists, and the checksum lists setup-ruby reads for releases
// newer than its catalog, so tests can resolve, download and unpack Ruby
// builds without the network.
package fakedist

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path"
	"strings"
	"sync"
	"testing"

	"github.com/circleci-functions/setup-ruby/rubydist"
)

// CatalogPath is where the server serves its catalog.
const CatalogPath = "/downloads.json"

// Release is one Ruby release the server has a build of.
type Release struct {
	Version string
	// Platform defaults to the host.
	Platform rubydist.Platform
	// Unlisted leaves the release out of the catalog, as one newer than
	// setup-ruby's would be, so it is found only through Live.
	Unlisted bool
}

// Server is a running fake distribution server.
type Server struct {
	URL string

	host     rubydist.Platform
	releases []Release
	mu       sync.Mutex
	requests []string
	corrupt  bool
}

// New starts a server with builds of releases, and stops it when t ends.
func New(t *testing.T, releases ...Release) *Server {
	t.Helper()
	host, err := rubydist.Host()
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{host: host}
	for _, rel := range releases {
		if rel.Platform == (rubydist.Platform{}) {
			rel.Platform = host
		}
		s.releases = append(s.releases, rel)
	}
	srv := httptest.NewServer(http.HandlerFunc(s.serve))
	t.Cleanup(srv.Close)
	s.URL = srv.URL
	return s
}

// CatalogURL is the URL of the server's catalog.
func (s *Server) CatalogURL() string {
	return s.URL + CatalogPath
}

// Live returns a Live that looks for unlisted releases on the server.
func (s *Server) Live() rubydist.Live {
	return rubydist.Live{JDX: s.URL + "/jdx", RV: s.URL + "/rv"}
}

// Corrupt makes the server send archives that do not match their checksums.
func (s *Server) Corrupt() {
	s.corrupt = true
}

// Downloads returns how many archives have been served.
func (s *Server) Downloads() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, p := range s.requests {
		if strings.HasSuffix(p, ".tar.gz") {
			n++
		}
	}
	return n
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.requests = append(s.requests, r.URL.Path)
	s.mu.Unlock()

	switch {
	case r.URL.Path == CatalogPath:
		s.serveCatalog(w)
		return
	case r.URL.Path == "/rv/SHA256SUMS":
		s.serveSums(w)
		return
	case strings.HasSuffix(r.URL.Path, ".intoto.jsonl"):
		s.serveProvenance(w, r.URL.Path)
		return
	}
	for _, rel := range s.releases {
		if s.Build(rel).URL != s.URL+r.URL.Path {
			continue
		}
		data := Archive(rel)
		if s.corrupt {
			data = append(data, 0)
		}
		_, _ = w.Write(data)
		return
	}
	http.NotFound(w, r)
}

func (s *Server) serveCatalog(w http.ResponseWriter) {
	var c rubydist.Catalog
	for _, rel := range s.releases {
		if !rel.Unlisted {
			c.Rubies = append(c.Rubies, s.Build(rel))
		}
	}
	data, _ := c.Marshal()
	_, _ = w.Write(data)
}

func (s *Server) serveSums(w http.ResponseWriter) {
	for _, rel := range s.releases {
		if asset, ok := rubydist.RVAsset(rel.Version, rel.Platform); ok {
			fmt.Fprintf(w, "%s  %s\n", checksum(rel), asset)
		}
	}
}

// serveProvenance serves a release's provenance as jdx/ruby does: a Sigstore
// bundle whose DSSE payload is an in-toto statement listing its archives.
func (s *Server) serveProvenance(w http.ResponseWriter, urlPath string) {
	type subject struct {
		Name   string            `json:"name"`
		Digest map[string]string `json:"digest"`
	}
	var subjects []subject
	for _, rel := range s.releases {
		asset, ok := rubydist.JDXAsset(rel.Version, rel.Platform)
		if ok && path.Base(urlPath) == "ruby-"+rel.Version+".intoto.jsonl" {
			subjects = append(subjects, subject{Name: asset, Digest: map[string]string{"sha256": checksum(rel)}})
		}
	}
	if len(subjects) == 0 {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	payload, _ := json.Marshal(map[string]any{"subject": subjects})
	_ = json.NewEncoder(w).Encode(map[string]any{
		"dsseEnvelope": map[string]string{"payload": base64.StdEncoding.EncodeToString(payload)},
	})
}

// Build returns the catalog entry the server lists for rel, as Live would
// also find it.
func (s *Server) Build(rel Release) rubydist.Build {
	if rel.Platform == (rubydist.Platform{}) {
		rel.Platform = s.host
	}
	return rubydist.Build{Version: rel.Version, Platform: rel.Platform.String(), URL: s.url(rel), SHA256: checksum(rel), Root: root(rel)}
}

// url is where the server serves rel's archive: where Live looks for it.
func (s *Server) url(rel Release) string {
	if asset, ok := rubydist.RVAsset(rel.Version, rel.Platform); ok {
		return s.URL + "/rv/" + asset
	}
	asset, ok := rubydist.JDXAsset(rel.Version, rel.Platform)
	if !ok {
		asset = fmt.Sprintf("ruby-%s.%s.tar.gz", rel.Version, rel.Platform)
	}
	return fmt.Sprintf("%s/jdx/%s/%s", s.URL, rel.Version, asset)
}

func root(rel Release) string {
	if rel.Platform.Libc == "musl" {
		return rubydist.RVRoot(rel.Version)
	}
	return rubydist.JDXRoot(rel.Version)
}

func checksum(rel Release) string {
	sum := sha256.Sum256(Archive(rel))
	return hex.EncodeToString(sum[:])
}

// Archive returns the archive of rel's build, laid out like a real one under
// its root directory. Its executables are shell scripts that report versions
// as the real ones do, and append any other invocation to $STUB_LOG, so a
// test can run them.
func Archive(rel Release) []byte {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	prefix := root(rel) + "/"
	file := func(name, content string, mode int64) {
		_ = tw.WriteHeader(&tar.Header{Name: prefix + name, Typeflag: tar.TypeReg, Mode: mode, Size: int64(len(content))})
		_, _ = tw.Write([]byte(content))
	}
	dir := func(name string) {
		_ = tw.WriteHeader(&tar.Header{Name: prefix + name, Typeflag: tar.TypeDir, Mode: 0755})
	}

	for _, d := range []string{"", "bin/", "lib/", "lib/ruby/"} {
		dir(d)
	}
	file("bin/ruby", script("ruby", "ruby "+strings.ReplaceAll(rel.Version, "-", "")+" (2026-01-01 revision 0000000000) +PRISM [fake]"), 0755)
	file("bin/gem", script("gem", "3.6.9"), 0755)
	file("bin/bundle", script("bundle", "Bundler version 2.6.9"), 0755)
	for _, name := range []string{"bundler", "irb", "rake", "erb", "rdoc", "ri"} {
		file("bin/"+name, script(name, name), 0755)
	}

	_ = tw.Close()
	_ = gz.Close()
	return buf.Bytes()
}

// script is a stand-in for one of a Ruby install's executables: it prints
// version for --version, -v or version, and otherwise logs how it was run.
// gem list fails, as it does when what it is asked about is not installed.
func script(name, version string) string {
	return `#!/bin/sh
[ "$1" = list ] && printf '` + name + ` %s\n' "$*" >> "${STUB_LOG:-/dev/null}" && exit 1
case "$1" in
--version|-v|version) echo "` + version + `" ;;
*) [ -n "$STUB_LOG" ] && printf '` + name + ` %s\n' "$*" >> "$STUB_LOG" ;;
esac
exit 0
`
}
