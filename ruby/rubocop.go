package ruby

import (
	"strconv"
	"strings"
)

// RubocopArgs builds the `bundle` args to run RuboCop. When parallel is true it
// adds --parallel; otherwise it adds --no-parallel for RuboCop >= 1.13 (which
// introduced the flag) and no parallelism flag for older versions. version is
// the output of `rubocop -v`.
func RubocopArgs(checkPath, format, outFile string, parallel bool, version string) []string {
	args := []string{"exec", "rubocop", checkPath, "--format", format}
	switch {
	case parallel:
		args = append(args, "--parallel")
	case rubocopAtLeast(version, 1, 13):
		args = append(args, "--no-parallel")
	}
	return append(args, "--out", outFile)
}

func rubocopAtLeast(version string, major, minor int) bool {
	fields := strings.Fields(version)
	if len(fields) == 0 {
		return false
	}
	parts := strings.Split(fields[0], ".")
	gotMajor := atoi(parts[0])
	gotMinor := 0
	if len(parts) > 1 {
		gotMinor = atoi(parts[1])
	}
	return gotMajor > major || (gotMajor == major && gotMinor >= minor)
}

func atoi(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	return n
}
