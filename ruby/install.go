package ruby

import "strings"

// RVM signing keys, trusted per https://rvm.io/rvm/security.
const (
	RVMKeyMpapis     = "409B6B1796C275462A1703113804BB82D39DC0E3"
	RVMKeyPkuczynski = "7D2BAF1CF37B13E2069D6956105BD0E739499BDB"
)

// GPGKeyservers are tried in order when importing the RVM signing keys.
var GPGKeyservers = []string{
	"keys.openpgp.org",
	"hkp://keyserver.ubuntu.com:80",
	"keyserver.ubuntu.com",
	"ha.pool.sks-keyservers.net",
	"hkp://ha.pool.sks-keyservers.net:80",
	"p80.pool.sks-keyservers.net",
	"hkp://p80.pool.sks-keyservers.net:80",
	"pgp.mit.edu",
	"hkp://pgp.mit.edu:80",
}

// NeedsOldOpenSSL reports whether the Ruby version needs the OpenSSL 1.1
// workaround on Linux, which applies to Ruby 3.0 and earlier.
func NeedsOldOpenSSL(version string) bool {
	major, minor := rubyMajorMinor(version)
	return major <= 2 || (major == 3 && minor == 0)
}

func rubyMajorMinor(v string) (int, int) {
	parts := strings.Split(strings.TrimSpace(v), ".")
	major := atoi(parts[0])
	minor := 0
	if len(parts) > 1 {
		minor = atoi(parts[1])
	}
	return major, minor
}
