package rubydist

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

// Live finds builds of Ruby releases newer than the catalog built into
// setup-ruby, where they are published, without the GitHub API and so
// without its rate limit.
type Live struct {
	// JDX is the base URL of jdx/ruby's release downloads, under which each
	// release has a tag named for its version.
	JDX string
	// RV is the URL of rv-ruby's latest release downloads, which hold every
	// version it builds.
	RV string
}

// DefaultLive looks where the builds are published.
var DefaultLive = Live{
	JDX: "https://github.com/" + JDXRepo + "/releases/download",
	RV:  "https://github.com/" + RVRepo + "/releases/latest/download",
}

// Find returns the build of Ruby version for p, with the checksum its
// publisher lists for it.
func (l Live) Find(ctx context.Context, p Platform, version string) (Build, error) {
	if asset, ok := JDXAsset(version, p); ok {
		base := fmt.Sprintf("%s/%s/", l.JDX, version)
		sum, err := provenanceDigest(ctx, base+"ruby-"+version+".intoto.jsonl", asset)
		if err != nil {
			return Build{}, err
		}
		return Build{Version: version, Platform: p.String(), URL: base + asset, SHA256: sum, Root: JDXRoot(version)}, nil
	}
	if asset, ok := RVAsset(version, p); ok {
		sum, err := listedDigest(ctx, l.RV+"/SHA256SUMS", asset)
		if err != nil {
			return Build{}, err
		}
		return Build{Version: version, Platform: p.String(), URL: l.RV + "/" + asset, SHA256: sum, Root: RVRoot(version)}, nil
	}
	return Build{}, fmt.Errorf("no published list of builds for %s to look for Ruby %s in", p, version)
}

// provenanceDigest returns the SHA-256 the SLSA provenance at url records for
// asset. Each line of the file is a Sigstore bundle whose DSSE payload is an
// in-toto statement naming the release's archives as its subjects.
func provenanceDigest(ctx context.Context, url, asset string) (string, error) {
	data, err := get(ctx, url)
	if err != nil {
		return "", err
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(nil, len(data)+1)
	for scanner.Scan() {
		var bundle struct {
			DSSEEnvelope struct {
				Payload string `json:"payload"`
			} `json:"dsseEnvelope"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &bundle); err != nil {
			continue
		}
		payload, err := base64.StdEncoding.DecodeString(bundle.DSSEEnvelope.Payload)
		if err != nil {
			continue
		}
		var statement struct {
			Subject []struct {
				Name   string            `json:"name"`
				Digest map[string]string `json:"digest"`
			} `json:"subject"`
		}
		if err := json.Unmarshal(payload, &statement); err != nil {
			continue
		}
		for _, s := range statement.Subject {
			if s.Name == asset && s.Digest["sha256"] != "" {
				return s.Digest["sha256"], nil
			}
		}
	}
	return "", fmt.Errorf("%s records no SHA-256 for %s", url, asset)
}

// listedDigest returns the SHA-256 the sha256sum-style list at url gives for
// asset.
func listedDigest(ctx context.Context, url, asset string) (string, error) {
	data, err := get(ctx, url)
	if err != nil {
		return "", err
	}
	for line := range strings.Lines(string(data)) {
		fields := strings.Fields(line)
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == asset {
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("%s lists no SHA-256 for %s", url, asset)
}
