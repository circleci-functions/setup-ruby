package main

import (
	"context"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/circleci-functions/setup-ruby/internal/testing/fakedist"
	"github.com/circleci-functions/setup-ruby/rubydist"
	"github.com/circleci-functions/setup-ruby/rubyversion"
)

var linux = rubydist.Platform{OS: "linux", Arch: "x86_64", Libc: "gnu"}

// fakeLive points the lookup for releases newer than the catalog at a fake
// server holding releases.
func fakeLive(t *testing.T, releases ...fakedist.Release) *fakedist.Server {
	t.Helper()
	srv := fakedist.New(t, releases...)
	old := live
	live = srv.Live()
	t.Cleanup(func() { live = old })
	return srv
}

func resolveSpec(t *testing.T, raw string) (rubydist.Build, error) {
	t.Helper()
	spec, err := rubyversion.Parse(raw)
	assert.NilError(t, err)
	return (&installCmd{}).resolve(context.Background(), spec, linux)
}

func TestResolve_UsesTheBuiltInList(t *testing.T) {
	srv := fakeLive(t)

	build, err := resolveSpec(t, "3.4.1")
	assert.NilError(t, err)
	assert.Equal(t, build.Version, "3.4.1")
	assert.Equal(t, srv.Downloads(), 0)
}

func TestResolve_LooksForANewerExactRelease(t *testing.T) {
	rel := fakedist.Release{Version: "3.99.1", Platform: linux, Unlisted: true}
	srv := fakeLive(t, rel)

	build, err := resolveSpec(t, "3.99.1")
	assert.NilError(t, err)
	assert.DeepEqual(t, build, srv.Build(rel))
}

func TestResolve_NoMatch(t *testing.T) {
	fakeLive(t, fakedist.Release{Version: "3.99.1", Platform: linux, Unlisted: true})

	tests := []struct {
		raw     string
		wantErr string
	}{
		{raw: "3.99", wantErr: "upgrade setup-ruby for newer releases"},
		{raw: "3.99.2", wantErr: "3.99.2.intoto.jsonl"},
	}
	for _, tc := range tests {
		t.Run(tc.raw, func(t *testing.T) {
			_, err := resolveSpec(t, tc.raw)
			assert.ErrorIs(t, err, rubyversion.ErrNoMatch)
			assert.ErrorContains(t, err, tc.wantErr)
		})
	}
}
