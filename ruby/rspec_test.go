package ruby

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestRspecExtraArgs(t *testing.T) {
	tests := []struct {
		name  string
		order string
		tag   string
		want  []string
	}{
		{name: "none", want: nil},
		{name: "order only", order: "random", want: []string{"--order", "random"}},
		{name: "single tag", tag: "focus", want: []string{"--tag", "focus"}},
		{name: "multiple tags", tag: "focus ~slow", want: []string{"--tag", "focus", "--tag", "~slow"}},
		{name: "order and tags", order: "defined", tag: "a b", want: []string{"--order", "defined", "--tag", "a", "--tag", "b"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.DeepEqual(t, RspecExtraArgs(tc.order, tc.tag), tc.want)
		})
	}
}

func TestRspecRerunCommand(t *testing.T) {
	got := RspecRerunCommand("/tmp/test-results/rspec", "42", nil)
	want := `xargs -t bundle exec rspec --seed "42" --profile 10 --format RspecJunitFormatter --out "/tmp/test-results/rspec"/results.xml --format progress`
	assert.Equal(t, got, want)

	got = RspecRerunCommand("/out", "7", []string{"--tag", "focus"})
	assert.Equal(t, got, `xargs -t bundle exec rspec --seed "7" --profile 10 --format RspecJunitFormatter --out "/out"/results.xml --format progress --tag focus`)
}

func TestRspecArgs(t *testing.T) {
	got := RspecArgs("/out", "9", []string{"spec/a_spec.rb", "spec/b_spec.rb"}, []string{"--tag", "focus"})
	want := []string{
		"exec", "rspec", "--seed", "9",
		"spec/a_spec.rb", "spec/b_spec.rb",
		"--profile", "10", "--format", "RspecJunitFormatter", "--out", "/out/results.xml", "--format", "progress",
		"--tag", "focus",
	}
	assert.DeepEqual(t, got, want)
}
