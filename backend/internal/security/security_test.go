package security

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if hash == "correct horse battery staple" {
		t.Fatal("password stored in plaintext")
	}

	ok, err := VerifyPassword("correct horse battery staple", hash)
	if err != nil || !ok {
		t.Fatalf("verify valid: ok=%v err=%v", ok, err)
	}

	ok, err = VerifyPassword("wrong", hash)
	if err != nil || ok {
		t.Fatalf("verify wrong: ok=%v err=%v", ok, err)
	}
}

func TestVerifyPassword_badFormat(t *testing.T) {
	if _, err := VerifyPassword("x", "not-a-phc-string"); err != ErrInvalidHash {
		t.Fatalf("want ErrInvalidHash, got %v", err)
	}
}

func TestAccessTokenRoundTrip(t *testing.T) {
	tm, err := NewTokenManager("test-secret", 15*time.Minute, time.Hour)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	id := uuid.New()
	now := time.Now()
	tok, err := tm.IssueAccess(id, now)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	got, err := tm.ParseAccess(tok)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got != id {
		t.Fatalf("subject mismatch: %v != %v", got, id)
	}
}

func TestParseAccess_expired(t *testing.T) {
	tm, _ := NewTokenManager("test-secret", time.Minute, time.Hour)
	tok, _ := tm.IssueAccess(uuid.New(), time.Now().Add(-2*time.Hour))
	if _, err := tm.ParseAccess(tok); err == nil {
		t.Fatal("expected expired token to fail")
	}
}

func TestParseAccess_wrongSecret(t *testing.T) {
	a, _ := NewTokenManager("secret-a", time.Minute, time.Hour)
	b, _ := NewTokenManager("secret-b", time.Minute, time.Hour)
	tok, _ := a.IssueAccess(uuid.New(), time.Now())
	if _, err := b.ParseAccess(tok); err == nil {
		t.Fatal("expected wrong-secret token to fail")
	}
}

func TestRefreshTokenHashing(t *testing.T) {
	tok, hash, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if tok == hash {
		t.Fatal("raw token must differ from stored hash")
	}
	if HashRefreshToken(tok) != hash {
		t.Fatal("hash not reproducible")
	}
}

func TestAPITokenHashing(t *testing.T) {
	tok, hash, err := GenerateAPIToken()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if tok == hash {
		t.Fatal("raw token must differ from stored hash")
	}
	if len(tok) < 4 || tok[:3] != "hp_" {
		t.Fatalf("api token must carry hp_ prefix, got %q", tok)
	}
	if HashAPIToken(tok) != hash {
		t.Fatal("hash not reproducible")
	}
	tok2, _, _ := GenerateAPIToken()
	if tok2 == tok {
		t.Fatal("tokens must be unique")
	}
}
