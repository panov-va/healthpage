// Package acme выпускает и продлевает TLS-сертификаты кастомных доменов через ACME (Let's Encrypt)
// по HTTP-01 (этап 4.3.2). Challenge'и кладутся в БД, edge-прокси (4.3.3) отдаёт их на :80.
// Реальный выпуск требует публичной доступности домена на :80 — отлаживается на прод-деплое.
package acme

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/registration"

	"github.com/healthpage/backend/internal/store"
)

// CertStore — зависимость менеджера от хранилища (БД).
type CertStore interface {
	ACMEAccount(ctx context.Context, directoryURL string) (store.ACMEAccount, error)
	SaveACMEAccount(ctx context.Context, directoryURL string, acc store.ACMEAccount) error
	DomainCertificate(ctx context.Context, domain string) (store.DomainCertificate, error)
	SaveDomainCertificate(ctx context.Context, c store.DomainCertificate) error
	PutACMEChallenge(ctx context.Context, token, keyAuth, domain string) error
	DeleteACMEChallenge(ctx context.Context, token string) error
	ListVerifiedDomains(ctx context.Context) ([]string, error)
}

// Manager выпускает/продлевает сертификаты. Потокобезопасен для последовательного использования
// из renewal-loop (не предполагает конкурентный Obtain одного домена).
type Manager struct {
	store        CertStore
	email        string
	directoryURL string
}

// New собирает менеджера. directoryURL — ACME-директория (LE prod/staging).
func New(st CertStore, email, directoryURL string) *Manager {
	return &Manager{store: st, email: email, directoryURL: directoryURL}
}

// dbChallengeProvider реализует lego challenge.Provider для HTTP-01: складывает token→keyAuth в
// БД (Present) и удаляет после проверки (CleanUp). Отдаёт challenge edge-прокси.
type dbChallengeProvider struct {
	ctx   context.Context
	store CertStore
}

func (p *dbChallengeProvider) Present(domain, token, keyAuth string) error {
	return p.store.PutACMEChallenge(p.ctx, token, keyAuth, domain)
}

func (p *dbChallengeProvider) CleanUp(_, token, _ string) error {
	return p.store.DeleteACMEChallenge(p.ctx, token)
}

// client строит lego-клиент с загруженным/созданным аккаунтом и HTTP-01 провайдером из БД.
func (m *Manager) client(ctx context.Context) (*lego.Client, error) {
	user, err := m.loadOrCreateUser(ctx)
	if err != nil {
		return nil, err
	}
	cfg := lego.NewConfig(user)
	cfg.CADirURL = m.directoryURL
	cfg.Certificate.KeyType = certcrypto.EC256

	cl, err := lego.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("acme: new client: %w", err)
	}
	if err := cl.Challenge.SetHTTP01Provider(&dbChallengeProvider{ctx: ctx, store: m.store}); err != nil {
		return nil, fmt.Errorf("acme: set http01 provider: %w", err)
	}

	// Регистрация аккаунта при первом запуске (нет сохранённой registration).
	if user.reg == nil {
		reg, err := cl.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
		if err != nil {
			return nil, fmt.Errorf("acme: register account: %w", err)
		}
		user.reg = reg
		if err := m.saveUser(ctx, user); err != nil {
			return nil, err
		}
	}
	return cl, nil
}

// Obtain выпускает (или перевыпускает) сертификат для домена и сохраняет его в БД.
func (m *Manager) Obtain(ctx context.Context, domain string) error {
	cl, err := m.client(ctx)
	if err != nil {
		return err
	}
	res, err := cl.Certificate.Obtain(certificate.ObtainRequest{
		Domains: []string{domain},
		Bundle:  true,
	})
	if err != nil {
		return fmt.Errorf("acme: obtain %s: %w", domain, err)
	}
	expires, err := certExpiry(res.Certificate)
	if err != nil {
		return fmt.Errorf("acme: parse cert %s: %w", domain, err)
	}
	return m.store.SaveDomainCertificate(ctx, store.DomainCertificate{
		Domain:    domain,
		CertPEM:   string(res.Certificate),
		KeyPEM:    string(res.PrivateKey),
		ExpiresAt: expires,
	})
}

// RenewDue выпускает сертификаты для верифицированных доменов без серта или с истечением раньше
// now+renewBefore. now передаётся явно (детерминизм/тесты). Возвращает число обработанных и
// первую ошибку (остальные домены всё равно обрабатываются).
func (m *Manager) RenewDue(ctx context.Context, now time.Time, renewBefore time.Duration) (int, error) {
	domains, err := m.store.ListVerifiedDomains(ctx)
	if err != nil {
		return 0, err
	}
	var firstErr error
	processed := 0
	for _, d := range domains {
		if !m.needsIssue(ctx, d, now, renewBefore) {
			continue
		}
		processed++
		if err := m.Obtain(ctx, d); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return processed, firstErr
}

// needsIssue: серта нет или он истекает в пределах renewBefore.
func (m *Manager) needsIssue(ctx context.Context, domain string, now time.Time, renewBefore time.Duration) bool {
	cert, err := m.store.DomainCertificate(ctx, domain)
	if err != nil {
		return true // нет серта (или ошибка чтения) → пытаемся выпустить
	}
	return now.Add(renewBefore).After(cert.ExpiresAt)
}

func certExpiry(certPEM []byte) (time.Time, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return time.Time{}, fmt.Errorf("no PEM block")
	}
	c, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return time.Time{}, err
	}
	return c.NotAfter, nil
}

// ── ACME-аккаунт (lego registration.User) ──

type acmeUser struct {
	email string
	key   crypto.PrivateKey
	reg   *registration.Resource
}

func (u *acmeUser) GetEmail() string                        { return u.email }
func (u *acmeUser) GetRegistration() *registration.Resource { return u.reg }
func (u *acmeUser) GetPrivateKey() crypto.PrivateKey        { return u.key }

func (m *Manager) loadOrCreateUser(ctx context.Context) (*acmeUser, error) {
	acc, err := m.store.ACMEAccount(ctx, m.directoryURL)
	if err == nil {
		key, perr := parseECKey(acc.PrivateKeyPEM)
		if perr != nil {
			return nil, fmt.Errorf("acme: parse account key: %w", perr)
		}
		var reg registration.Resource
		if uerr := json.Unmarshal(acc.RegistrationRaw, &reg); uerr != nil {
			return nil, fmt.Errorf("acme: parse registration: %w", uerr)
		}
		return &acmeUser{email: acc.Email, key: key, reg: &reg}, nil
	}
	// Нет аккаунта → генерируем ключ; регистрация произойдёт в client().
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("acme: generate account key: %w", err)
	}
	return &acmeUser{email: m.email, key: key}, nil
}

func (m *Manager) saveUser(ctx context.Context, u *acmeUser) error {
	keyPEM, err := marshalECKey(u.key)
	if err != nil {
		return err
	}
	regRaw, err := json.Marshal(u.reg)
	if err != nil {
		return fmt.Errorf("acme: marshal registration: %w", err)
	}
	return m.store.SaveACMEAccount(ctx, m.directoryURL, store.ACMEAccount{
		Email: u.email, PrivateKeyPEM: keyPEM, RegistrationRaw: regRaw,
	})
}

func marshalECKey(key crypto.PrivateKey) (string, error) {
	ec, ok := key.(*ecdsa.PrivateKey)
	if !ok {
		return "", fmt.Errorf("acme: account key is not ECDSA")
	}
	der, err := x509.MarshalECPrivateKey(ec)
	if err != nil {
		return "", fmt.Errorf("acme: marshal ec key: %w", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der})), nil
}

func parseECKey(pemStr string) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, fmt.Errorf("acme: no PEM block in account key")
	}
	return x509.ParseECPrivateKey(block.Bytes)
}
