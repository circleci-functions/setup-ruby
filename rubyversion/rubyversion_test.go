package rubyversion

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestParse_Errors(t *testing.T) {
	tests := []struct {
		raw     string
		wantErr string
	}{
		{raw: "head", wantErr: "development builds are not supported"},
		{raw: "jruby-9.4.8.0", wantErr: "only CRuby is supported, not jruby"},
		{raw: "truffleruby", wantErr: "only CRuby is supported, not truffleruby"},
		{raw: "truffleruby+graalvm-24.1.0", wantErr: "only CRuby is supported, not truffleruby+graalvm"},
		{raw: ">= 3.2", wantErr: "invalid ruby version"},
		{raw: "banana", wantErr: "invalid ruby version"},
		{raw: "3.4.1.2", wantErr: "invalid ruby version"},
	}
	for _, tc := range tests {
		t.Run(tc.raw, func(t *testing.T) {
			_, err := Parse(tc.raw)
			assert.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestResolve(t *testing.T) {
	versions := []string{
		"3.1.7", "3.2.0", "3.2.10", "3.2.9", "3.3.0-preview1", "3.3.0",
		"3.4.0-preview2", "3.4.0-rc1", "3.4.0", "3.4.1", "3.5.0-preview1",
	}
	tests := []struct {
		raw  string
		want string
	}{
		{raw: "3.2", want: "3.2.10"},
		{raw: "3.2.x", want: "3.2.10"},
		{raw: "3", want: "3.4.1"},
		{raw: "3.x", want: "3.4.1"},
		{raw: "3.4.0", want: "3.4.0"},
		{raw: "ruby-3.4.0", want: "3.4.0"},
		{raw: "3.4.0-rc1", want: "3.4.0-rc1"},
		{raw: "latest", want: "3.4.1"},
		{raw: "", want: "3.4.1"},
		{raw: "3.5", want: "3.5.0-preview1"},
		{raw: "3.1", want: "3.1.7"},
	}
	for _, tc := range tests {
		t.Run(tc.raw, func(t *testing.T) {
			spec, err := Parse(tc.raw)
			assert.NilError(t, err)
			got, err := spec.Resolve(versions)
			assert.NilError(t, err)
			assert.Equal(t, got, tc.want)
		})
	}
}

func TestResolve_NoMatch(t *testing.T) {
	versions := []string{"3.10.0", "3.4.1", "3.5.0-preview1"}
	for _, raw := range []string{"3.1", "3.4.2", "4"} {
		t.Run(raw, func(t *testing.T) {
			spec, err := Parse(raw)
			assert.NilError(t, err)
			_, err = spec.Resolve(versions)
			assert.ErrorIs(t, err, ErrNoMatch)
		})
	}
}

func TestResolve_LatestSkipsPrereleases(t *testing.T) {
	spec, err := Parse("latest")
	assert.NilError(t, err)
	_, err = spec.Resolve([]string{"3.5.0-preview1"})
	assert.ErrorIs(t, err, ErrNoMatch)
}

func TestCompare(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{a: "3.4.10", b: "3.4.9", want: 1},
		{a: "3.4.0", b: "3.4.0-rc1", want: 1},
		{a: "3.4.0-rc1", b: "3.4.0-preview2", want: 1},
		{a: "3.4.0-preview10", b: "3.4.0-preview9", want: 1},
		{a: "4.0.0-preview1", b: "3.4.10", want: 1},
		{a: "3.4.1", b: "3.4.1", want: 0},
	}
	for _, tc := range tests {
		t.Run(tc.a+" vs "+tc.b, func(t *testing.T) {
			assert.Equal(t, sign(Compare(tc.a, tc.b)), tc.want)
			assert.Equal(t, sign(Compare(tc.b, tc.a)), -tc.want)
		})
	}
}

func sign(n int) int {
	switch {
	case n > 0:
		return 1
	case n < 0:
		return -1
	}
	return 0
}
