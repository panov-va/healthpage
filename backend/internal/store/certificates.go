package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/healthpage/backend/internal/store/db"
)

// DomainCertificate — выпущенный TLS-сертификат кастомного домена (этап 4.3.2).
type DomainCertificate struct {
	Domain    string
	CertPEM   string
	KeyPEM    string
	ExpiresAt time.Time
}

// ACMEAccount — сохранённый ACME-аккаунт (ключ + регистрация) для lego.
type ACMEAccount struct {
	Email           string
	PrivateKeyPEM   string
	RegistrationRaw json.RawMessage
}

// DomainCertificate возвращает сертификат по домену. ErrNotFound если нет.
func (s *Store) DomainCertificate(ctx context.Context, domain string) (DomainCertificate, error) {
	row, err := s.q.GetDomainCertificate(ctx, domain)
	if err != nil {
		return DomainCertificate{}, wrapNotFound(err)
	}
	return DomainCertificate{Domain: row.Domain, CertPEM: row.CertPem, KeyPEM: row.KeyPem, ExpiresAt: row.ExpiresAt}, nil
}

// SaveDomainCertificate сохраняет/обновляет сертификат домена.
func (s *Store) SaveDomainCertificate(ctx context.Context, c DomainCertificate) error {
	if err := s.q.UpsertDomainCertificate(ctx, db.UpsertDomainCertificateParams{
		Domain: c.Domain, CertPem: c.CertPEM, KeyPem: c.KeyPEM, ExpiresAt: c.ExpiresAt,
	}); err != nil {
		return fmt.Errorf("store: save certificate: %w", err)
	}
	return nil
}

// DeleteDomainCertificate удаляет сертификат домена (например, при отвязке).
func (s *Store) DeleteDomainCertificate(ctx context.Context, domain string) error {
	if err := s.q.DeleteDomainCertificate(ctx, domain); err != nil {
		return fmt.Errorf("store: delete certificate: %w", err)
	}
	return nil
}

// ACMEAccount возвращает ACME-аккаунт для directory URL. ErrNotFound если нет.
func (s *Store) ACMEAccount(ctx context.Context, directoryURL string) (ACMEAccount, error) {
	row, err := s.q.GetACMEAccount(ctx, directoryURL)
	if err != nil {
		return ACMEAccount{}, wrapNotFound(err)
	}
	return ACMEAccount{Email: row.Email, PrivateKeyPEM: row.PrivateKey, RegistrationRaw: row.Registration}, nil
}

// SaveACMEAccount сохраняет ACME-аккаунт.
func (s *Store) SaveACMEAccount(ctx context.Context, directoryURL string, acc ACMEAccount) error {
	if err := s.q.UpsertACMEAccount(ctx, db.UpsertACMEAccountParams{
		DirectoryUrl: directoryURL, Email: acc.Email, PrivateKey: acc.PrivateKeyPEM, Registration: acc.RegistrationRaw,
	}); err != nil {
		return fmt.Errorf("store: save acme account: %w", err)
	}
	return nil
}

// PutACMEChallenge сохраняет активный HTTP-01 challenge (edge отдаёт его на :80).
func (s *Store) PutACMEChallenge(ctx context.Context, token, keyAuth, domain string) error {
	if err := s.q.PutACMEChallenge(ctx, db.PutACMEChallengeParams{Token: token, KeyAuth: keyAuth, Domain: domain}); err != nil {
		return fmt.Errorf("store: put acme challenge: %w", err)
	}
	return nil
}

// ACMEChallenge возвращает key authorization по токену. ErrNotFound если нет.
func (s *Store) ACMEChallenge(ctx context.Context, token string) (string, error) {
	keyAuth, err := s.q.GetACMEChallenge(ctx, token)
	if err != nil {
		return "", wrapNotFound(err)
	}
	return keyAuth, nil
}

// DeleteACMEChallenge удаляет challenge после завершения проверки.
func (s *Store) DeleteACMEChallenge(ctx context.Context, token string) error {
	if err := s.q.DeleteACMEChallenge(ctx, token); err != nil {
		return fmt.Errorf("store: delete acme challenge: %w", err)
	}
	return nil
}

// SlugByCustomDomain возвращает slug страницы по верифицированному кастомному домену (для
// роутинга edge-прокси, этап 4.3.3). ErrNotFound если домена нет/не верифицирован.
func (s *Store) SlugByCustomDomain(ctx context.Context, domain string) (string, error) {
	d := domain
	slug, err := s.q.GetSlugByCustomDomain(ctx, &d)
	if err != nil {
		return "", wrapNotFound(err)
	}
	return slug, nil
}

// ListVerifiedDomains возвращает домены страниц с подтверждённым CNAME (кандидаты на выпуск серта).
func (s *Store) ListVerifiedDomains(ctx context.Context) ([]string, error) {
	rows, err := s.q.ListVerifiedDomains(ctx)
	if err != nil {
		return nil, fmt.Errorf("store: list verified domains: %w", err)
	}
	out := make([]string, 0, len(rows))
	for _, d := range rows {
		if d != nil && *d != "" {
			out = append(out, *d)
		}
	}
	return out, nil
}
