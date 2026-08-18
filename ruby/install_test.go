package ruby

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestNeedsOldOpenSSL(t *testing.T) {
	tests := []struct {
		version string
		want    bool
	}{
		{"2.6.5", true},
		{"2.7.5", true},
		{"3.0.0", true},
		{"3.0", true},
		{"3.1.0", false},
		{"3.3", false},
		{"3.4.1", false},
		{"1.9.3", true},
	}

	for _, tc := range tests {
		t.Run(tc.version, func(t *testing.T) {
			assert.Equal(t, NeedsOldOpenSSL(tc.version), tc.want)
		})
	}
}
