package auth

import (
	"bytes"
	"testing"
)

func TestHashPassword_verifiesOnlyTheRightPassword(t *testing.T) {
	c, err := HashPassword("senha-de-teste")
	if err != nil {
		t.Fatal(err)
	}
	if !Verify(c, "senha-de-teste") {
		t.Error("a senha correta deveria verificar")
	}
	if Verify(c, "senha-de-teste ") {
		t.Error("senha com espaço a mais não pode verificar")
	}
	if Verify(c, "outra") {
		t.Error("senha errada não pode verificar")
	}
}

// A per-install salt is what keeps two archives with the same password from sharing a
// hash, so a precomputed table cannot cover both.
func TestHashPassword_saltIsPerCall(t *testing.T) {
	a, err := HashPassword("mesma-senha")
	if err != nil {
		t.Fatal(err)
	}
	b, err := HashPassword("mesma-senha")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(a.Salt, b.Salt) {
		t.Error("dois hashes da mesma senha usaram o mesmo salt")
	}
	if bytes.Equal(a.Hash, b.Hash) {
		t.Error("dois hashes da mesma senha saíram idênticos")
	}
}

// The zero value shows up whenever no password was set yet; it must reject everything
// instead of accepting an empty one.
func TestVerify_zeroCredentialRejectsEverything(t *testing.T) {
	for _, password := range []string{"", "qualquer"} {
		if Verify(Credential{}, password) {
			t.Errorf("credencial vazia aceitou %q", password)
		}
	}
}

func TestNewSessionToken_isRandomAndOnlyTheDigestLeaves(t *testing.T) {
	token, digest := NewSessionToken()
	other, _ := NewSessionToken()

	if token == "" || token == other {
		t.Error("tokens de sessão precisam ser aleatórios e distintos")
	}
	if bytes.Equal(digest, []byte(token)) {
		t.Error("o digest não pode ser o próprio token")
	}
	if !bytes.Equal(digest, TokenDigest(token)) {
		t.Error("TokenDigest precisa reproduzir o digest do token emitido")
	}
}

func TestGeneratePassword_isLongAndRandom(t *testing.T) {
	a, b := GeneratePassword(), GeneratePassword()
	if a == b {
		t.Error("duas senhas geradas saíram iguais")
	}
	// Short enough to retype, long enough that no attempt limiter is needed.
	if len(a) < 20 {
		t.Errorf("senha gerada tem %d caracteres, esperado ao menos 20", len(a))
	}
}
