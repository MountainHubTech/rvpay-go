package providers

import (
	"crypto/sha256"
	"encoding/hex"
)

// fingerprintPrefixLen is the number of hexadecimal characters logged from
// the SHA-256 digest of an access token.
const fingerprintPrefixLen = 12

// AccessTokenFingerprint returns a short, non-reversible fingerprint of an
// access token for safe diagnostic logging: the first 12 hexadecimal
// characters of the token's SHA-256 hash. The token itself is never returned,
// logged, or stored by this function, and the digest cannot be reversed into
// the token. An empty token yields an empty fingerprint so callers do not
// mistake a missing token for a real one.
func AccessTokenFingerprint(accessToken string) string {
	if accessToken == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(accessToken))
	return hex.EncodeToString(sum[:])[:fingerprintPrefixLen]
}
