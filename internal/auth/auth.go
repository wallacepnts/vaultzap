// Package auth hashes the login password and mints session tokens. It knows nothing about
// HTTP or storage: the caller passes strings in and gets bytes out.
package auth

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
)

// Iterations is what a new password is hashed with. PBKDF2-HMAC-SHA256 at OWASP's
// recommended count — it is in the standard library since Go 1.24, which is what makes a
// stored password possible here without pulling in bcrypt or argon2.
const Iterations = 600_000

const (
	saltSize = 16
	keySize  = 32
)

// Credential is a stored login: the username, the derived key, its salt, and the cost it
// was derived with. Iterations travels with it so raising the constant never locks anyone
// out.
type Credential struct {
	Username   string
	Hash       []byte
	Salt       []byte
	Iterations int
}

// SameUsername compares over digests, so neither the length nor the first differing byte
// shows up in the timing — the same reason withAuth hashes before comparing.
func SameUsername(a, b string) bool {
	x, y := sha256.Sum256([]byte(a)), sha256.Sum256([]byte(b))
	return subtle.ConstantTimeCompare(x[:], y[:]) == 1
}

// HashPassword derives a new credential with a fresh random salt.
func HashPassword(password string) (Credential, error) {
	salt := make([]byte, saltSize)
	if _, err := rand.Read(salt); err != nil {
		return Credential{}, fmt.Errorf("gerar salt: %w", err)
	}
	hash, err := pbkdf2.Key(sha256.New, password, salt, Iterations, keySize)
	if err != nil {
		return Credential{}, fmt.Errorf("derivar senha: %w", err)
	}
	return Credential{Hash: hash, Salt: salt, Iterations: Iterations}, nil
}

// Verify reports whether password derives to the stored hash, comparing in constant time.
func Verify(c Credential, password string) bool {
	if len(c.Hash) == 0 || c.Iterations <= 0 {
		return false
	}
	hash, err := pbkdf2.Key(sha256.New, password, c.Salt, c.Iterations, len(c.Hash))
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(hash, c.Hash) == 1
}

// GeneratePassword returns the password the app sets for itself on a fresh install.
// rand.Text is 26 base32 characters, around 130 bits — past anything a brute force over
// the network reaches, which is why no attempt limiter comes with this.
func GeneratePassword() string {
	return rand.Text()
}

// NewSessionToken returns the value for the cookie and the digest to store. Only the
// digest is ever persisted.
func NewSessionToken() (token string, digest []byte) {
	token = rand.Text()
	return token, TokenDigest(token)
}

// TokenDigest is how a cookie value is turned into the key stored in the sessions table.
func TokenDigest(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}
