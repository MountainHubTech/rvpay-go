package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Argon2id parameters. These are deliberately conservative production
// defaults: 64 MiB memory, 3 iterations, 4 parallelism lanes.
const (
	argonTime    = 3
	argonMemory  = 64 * 1024
	argonThreads = 4
	argonKeyLen  = 32
	argonSaltLen = 16
	// ArgonEncodedPrefix is the PHC-style prefix of stored hashes.
	ArgonEncodedPrefix = "$argon2id$"
)

// HashPassword derives an argon2id PHC-format hash for the given password.
// The plaintext password is never persisted or logged.
func HashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	encoded := fmt.Sprintf("%sv=%d$m=%d,t=%d,p=%d$%s$%s",
		ArgonEncodedPrefix, argon2.Version, argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	)
	return encoded, nil
}

// VerifyPassword checks a plaintext password against a stored argon2id
// PHC-format hash using a constant-time comparison. It returns false (not an
// error) for malformed hashes so unexpected hash formats behave exactly like
// wrong passwords.
func VerifyPassword(password, encoded string) bool {
	if !strings.HasPrefix(encoded, ArgonEncodedPrefix) {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(encoded, ArgonEncodedPrefix), "$")
	if len(parts) != 4 {
		return false
	}
	var version int
	if _, err := fmt.Sscanf(parts[0], "v=%d", &version); err != nil {
		return false
	}
	var memory, time uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[1], "m=%d,t=%d,p=%d", &memory, &time, &threads); err != nil {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil {
		return false
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return false
	}
	actual := argon2.IDKey([]byte(password), salt, time, memory, threads, uint32(len(expected)))
	return subtle.ConstantTimeCompare(actual, expected) == 1
}

// generateToken returns a cryptographically random 256-bit opaque token,
// URL-safe base64 encoded.
func generateToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// tokenHash maps a raw token to its stored digest. Only hashes are ever
// persisted or compared in the database; the raw token never leaves the
// request path.
func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
