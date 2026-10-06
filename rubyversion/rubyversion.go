// Package rubyversion parses the Ruby versions setup-ruby is asked for and
// picks the release each one resolves to.
package rubyversion

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Latest is the spec for the newest stable release.
const Latest = "latest"

// ErrNoMatch is returned by Resolve when no release matches the spec.
var ErrNoMatch = errors.New("no Ruby release matches")

// Spec is a version asked for: an exact release, a prefix of one, or the
// newest stable release.
type Spec struct {
	raw    string
	prefix string
	exact  bool
}

var (
	exactVersion   = regexp.MustCompile(`^\d+\.\d+\.\d+(-[a-z]+\d*)?$`)
	partialVersion = regexp.MustCompile(`^\d+(\.\d+)?$`)
	engineVersion  = regexp.MustCompile(`^(jruby|truffleruby(?:\+graalvm)?|mruby|rbx)(-|$)`)
)

// unsupported are names ruby/setup-ruby accepts for development builds,
// which are not relocatable and so have no build setup-ruby can install.
var unsupported = []string{"head", "debug", "mingw", "mswin", "ucrt", "asan", "asan-release"}

// Parse reads a spec: an exact release (3.4.1, 3.5.0-preview1), a major or
// major.minor prefix (3, 3.4, 3.4.x) or latest. A leading ruby- is dropped,
// as .ruby-version files often carry one.
func Parse(raw string) (Spec, error) {
	v := strings.ToLower(strings.TrimSpace(raw))
	v = strings.TrimPrefix(v, "ruby-")
	switch {
	case v == "" || v == Latest:
		return Spec{raw: raw}, nil
	case exactVersion.MatchString(v):
		return Spec{raw: raw, prefix: v, exact: true}, nil
	}
	trimmed := strings.TrimSuffix(strings.TrimSuffix(v, ".x"), ".*")
	if partialVersion.MatchString(trimmed) {
		return Spec{raw: raw, prefix: trimmed}, nil
	}
	if slices.Contains(unsupported, v) {
		return Spec{}, fmt.Errorf("ruby version %q: development builds are not supported; ask for a release such as 3.4", raw)
	}
	if m := engineVersion.FindStringSubmatch(v); m != nil {
		return Spec{}, fmt.Errorf("ruby version %q: only CRuby is supported, not %s", raw, m[1])
	}
	return Spec{}, fmt.Errorf("invalid ruby version %q: want an exact or partial version such as 3.4.1 or 3.4", raw)
}

func (s Spec) String() string {
	if s.raw == "" {
		return Latest
	}
	return s.raw
}

// Exact returns the release an exact spec names.
func (s Spec) Exact() (string, bool) {
	return s.prefix, s.exact
}

// Resolve returns the release in versions that s picks: the exact release it
// names, or the newest stable release it is a prefix of, falling back to the
// newest prerelease while a version has no stable release yet.
func (s Spec) Resolve(versions []string) (string, error) {
	if s.exact {
		if slices.Contains(versions, s.prefix) {
			return s.prefix, nil
		}
		return "", fmt.Errorf("%w %s", ErrNoMatch, s)
	}
	var stable, pre []string
	for _, v := range versions {
		if !s.matches(v) {
			continue
		}
		if Prerelease(v) {
			pre = append(pre, v)
		} else {
			stable = append(stable, v)
		}
	}
	switch {
	case len(stable) > 0:
		return slices.MaxFunc(stable, Compare), nil
	case len(pre) > 0 && s.prefix != "":
		return slices.MaxFunc(pre, Compare), nil
	}
	return "", fmt.Errorf("%w %s", ErrNoMatch, s)
}

// matches reports whether v is a release s is a prefix of, on a dot
// boundary, so that 3.1 does not match 3.10.0.
func (s Spec) matches(v string) bool {
	if s.prefix == "" {
		return true
	}
	return v == s.prefix || strings.HasPrefix(v, s.prefix+".")
}

// Prerelease reports whether v is a preview or release candidate.
func Prerelease(v string) bool {
	return strings.Contains(v, "-")
}

// Compare orders two releases, as cmp.Compare does: by version number, then
// a prerelease before the release, then prereleases by name and number.
func Compare(a, b string) int {
	an, apre, _ := strings.Cut(a, "-")
	bn, bpre, _ := strings.Cut(b, "-")
	if c := compareNumbers(an, bn); c != 0 {
		return c
	}
	switch {
	case apre == bpre:
		return 0
	case apre == "":
		return 1
	case bpre == "":
		return -1
	}
	return compareSuffix(apre, bpre)
}

func compareNumbers(a, b string) int {
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := range max(len(as), len(bs)) {
		if c := number(as, i) - number(bs, i); c != 0 {
			return c
		}
	}
	return 0
}

func number(parts []string, i int) int {
	if i >= len(parts) {
		return 0
	}
	n, _ := strconv.Atoi(parts[i])
	return n
}

var suffix = regexp.MustCompile(`^([a-z]*)(\d*)$`)

// compareSuffix orders prerelease suffixes such as preview2 and rc1: by name,
// which puts preview before rc, then by number.
func compareSuffix(a, b string) int {
	am, bm := suffix.FindStringSubmatch(a), suffix.FindStringSubmatch(b)
	if am == nil || bm == nil {
		return strings.Compare(a, b)
	}
	if c := strings.Compare(am[1], bm[1]); c != 0 {
		return c
	}
	an, _ := strconv.Atoi(am[2])
	bn, _ := strconv.Atoi(bm[2])
	return an - bn
}
