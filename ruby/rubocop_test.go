package ruby

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestRubocopArgs(t *testing.T) {
	tests := []struct {
		name     string
		parallel bool
		version  string
		want     []string
	}{
		{
			name:     "parallel",
			parallel: true,
			version:  "1.50.2",
			want:     []string{"exec", "rubocop", ".", "--format", "progress", "--parallel", "--out", "/out/check.xml"},
		},
		{
			name:    "no parallel modern rubocop",
			version: "1.50.2",
			want:    []string{"exec", "rubocop", ".", "--format", "progress", "--no-parallel", "--out", "/out/check.xml"},
		},
		{
			name:    "no parallel old rubocop omits flag",
			version: "1.12.1",
			want:    []string{"exec", "rubocop", ".", "--format", "progress", "--out", "/out/check.xml"},
		},
		{
			name:    "no parallel boundary 1.13",
			version: "1.13.0",
			want:    []string{"exec", "rubocop", ".", "--format", "progress", "--no-parallel", "--out", "/out/check.xml"},
		},
		{
			name:    "version with trailing text",
			version: "2.0.0 (using Parser 3.2)",
			want:    []string{"exec", "rubocop", ".", "--format", "progress", "--no-parallel", "--out", "/out/check.xml"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := RubocopArgs(".", "progress", "/out/check.xml", tc.parallel, tc.version)
			assert.DeepEqual(t, got, tc.want)
		})
	}
}
