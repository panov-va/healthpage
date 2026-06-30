package acme

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/healthpage/backend/internal/store"
)

// fakeStore — in-memory CertStore для юнит-тестов (без БД/сети).
type fakeStore struct {
	certs      map[string]store.DomainCertificate
	challenges map[string]string
	verified   []string
}

func newFakeStore() *fakeStore {
	return &fakeStore{certs: map[string]store.DomainCertificate{}, challenges: map[string]string{}}
}

func (f *fakeStore) ACMEAccount(context.Context, string) (store.ACMEAccount, error) {
	return store.ACMEAccount{}, store.ErrNotFound
}
func (f *fakeStore) SaveACMEAccount(context.Context, string, store.ACMEAccount) error { return nil }
func (f *fakeStore) DomainCertificate(_ context.Context, d string) (store.DomainCertificate, error) {
	c, ok := f.certs[d]
	if !ok {
		return store.DomainCertificate{}, store.ErrNotFound
	}
	return c, nil
}
func (f *fakeStore) SaveDomainCertificate(_ context.Context, c store.DomainCertificate) error {
	f.certs[c.Domain] = c
	return nil
}
func (f *fakeStore) PutACMEChallenge(_ context.Context, token, keyAuth, _ string) error {
	f.challenges[token] = keyAuth
	return nil
}
func (f *fakeStore) DeleteACMEChallenge(_ context.Context, token string) error {
	delete(f.challenges, token)
	return nil
}
func (f *fakeStore) ListVerifiedDomains(context.Context) ([]string, error) { return f.verified, nil }

func TestNeedsIssue(t *testing.T) {
	st := newFakeStore()
	m := New(st, "ops@acme.test", "https://example/dir")
	now := time.Unix(1_700_000_000, 0)
	renewBefore := 30 * 24 * time.Hour

	// нет серта → нужно выпускать
	if !m.needsIssue(context.Background(), "a.test", now, renewBefore) {
		t.Error("ожидался выпуск при отсутствии серта")
	}
	// свежий серт (истекает через 60д) → не нужно
	st.certs["b.test"] = store.DomainCertificate{Domain: "b.test", ExpiresAt: now.Add(60 * 24 * time.Hour)}
	if m.needsIssue(context.Background(), "b.test", now, renewBefore) {
		t.Error("свежий серт не должен продлеваться")
	}
	// истекает через 10д (< renewBefore) → нужно
	st.certs["c.test"] = store.DomainCertificate{Domain: "c.test", ExpiresAt: now.Add(10 * 24 * time.Hour)}
	if !m.needsIssue(context.Background(), "c.test", now, renewBefore) {
		t.Error("истекающий серт должен продлеваться")
	}
}

func TestChallengeProvider(t *testing.T) {
	st := newFakeStore()
	p := &dbChallengeProvider{ctx: context.Background(), store: st}
	if err := p.Present("a.test", "tok", "keyauth"); err != nil {
		t.Fatalf("Present: %v", err)
	}
	if st.challenges["tok"] != "keyauth" {
		t.Fatalf("challenge не сохранён: %v", st.challenges)
	}
	if err := p.CleanUp("a.test", "tok", "keyauth"); err != nil {
		t.Fatalf("CleanUp: %v", err)
	}
	if _, ok := st.challenges["tok"]; ok {
		t.Fatal("challenge не удалён")
	}
}

func TestCertExpiry(t *testing.T) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	notAfter := time.Now().Add(90 * 24 * time.Hour).Truncate(time.Second)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "a.test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     notAfter,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})

	got, err := certExpiry(certPEM)
	if err != nil {
		t.Fatalf("certExpiry: %v", err)
	}
	if !got.Equal(notAfter) {
		t.Fatalf("expiry: got %v want %v", got, notAfter)
	}

	if _, err := certExpiry([]byte("not a pem")); err == nil {
		t.Error("ожидалась ошибка на мусоре")
	}
}

// TestKeyRoundTrip — сериализация/парсинг ключа аккаунта (используется при сохранении в БД).
func TestKeyRoundTrip(t *testing.T) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	pemStr, err := marshalECKey(key)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	parsed, err := parseECKey(pemStr)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !key.Equal(parsed) {
		t.Fatal("ключ не совпал после round-trip")
	}
	if _, err := parseECKey("garbage"); err == nil {
		t.Error("ожидалась ошибка на мусоре")
	}
}
