// Package edge — обратный прокси для кастомных доменов (этап 4.3.3): терминация TLS по SNI
// (сертификаты из БД, выпущенные tls-manager 4.3.2), отдача HTTP-01 challenge на :80 и роутинг
// по Host на public-ssr (резолв домен→страница) и API. Реальная отладка — на прод-деплое
// (нужны публичный DNS, доступность :80/:443 и выпущенные сертификаты).
package edge

import (
	"context"
	"crypto/tls"
	"errors"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/healthpage/backend/internal/store"
)

const acmeChallengePrefix = "/.well-known/acme-challenge/"

// Resolver — зависимости edge от хранилища.
type Resolver interface {
	DomainCertificate(ctx context.Context, domain string) (store.DomainCertificate, error)
	ACMEChallenge(ctx context.Context, token string) (string, error)
	SlugByCustomDomain(ctx context.Context, domain string) (string, error)
}

// Server — edge-прокси. apiProxy/ssrProxy — обратные прокси к соответствующим бэкендам.
type Server struct {
	resolver Resolver
	apiProxy *httputil.ReverseProxy
	ssrProxy *httputil.ReverseProxy

	mu    sync.Mutex
	certs map[string]*tls.Certificate // кэш разобранных сертификатов по домену
}

// New собирает edge-прокси. apiURL — origin API (для /api/*), ssrURL — origin public-ssr.
func New(resolver Resolver, apiURL, ssrURL string) (*Server, error) {
	api, err := url.Parse(apiURL)
	if err != nil {
		return nil, err
	}
	ssr, err := url.Parse(ssrURL)
	if err != nil {
		return nil, err
	}
	return &Server{
		resolver: resolver,
		apiProxy: httputil.NewSingleHostReverseProxy(api),
		ssrProxy: httputil.NewSingleHostReverseProxy(ssr),
		certs:    map[string]*tls.Certificate{},
	}, nil
}

// TLSConfig — конфиг для :443: сертификат выбирается по SNI из БД.
func (s *Server) TLSConfig() *tls.Config {
	return &tls.Config{GetCertificate: s.getCertificate, MinVersion: tls.VersionTLS12}
}

// getCertificate возвращает сертификат для запрошенного домена (SNI), кэшируя разобранный.
func (s *Server) getCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	domain := strings.ToLower(hello.ServerName)
	if domain == "" {
		return nil, errors.New("edge: no SNI server name")
	}

	s.mu.Lock()
	if c, ok := s.certs[domain]; ok && certValid(c) {
		s.mu.Unlock()
		return c, nil
	}
	s.mu.Unlock()

	rec, err := s.resolver.DomainCertificate(hello.Context(), domain)
	if err != nil {
		return nil, err
	}
	cert, err := tls.X509KeyPair([]byte(rec.CertPEM), []byte(rec.KeyPEM))
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.certs[domain] = &cert
	s.mu.Unlock()
	return &cert, nil
}

// certValid — у кэшированного серта есть валидный leaf и он не истёк (с запасом 24ч).
func certValid(c *tls.Certificate) bool {
	return c.Leaf == nil || time.Now().Add(24*time.Hour).Before(c.Leaf.NotAfter)
}

// HTTPHandler — обработчик :80: отдаёт HTTP-01 challenge, остальное редиректит на HTTPS.
func (s *Server) HTTPHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, acmeChallengePrefix) {
			token := strings.TrimPrefix(r.URL.Path, acmeChallengePrefix)
			keyAuth, err := s.resolver.ACMEChallenge(r.Context(), token)
			if err != nil {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte(keyAuth))
			return
		}
		target := "https://" + hostOnly(r.Host) + r.URL.RequestURI()
		http.Redirect(w, r, target, http.StatusMovedPermanently)
	})
}

// HTTPSHandler — обработчик :443: /api/* → API, остальное → public-ssr с резолвом домен→страница.
func (s *Server) HTTPSHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isAPIPath(r.URL.Path) {
			s.apiProxy.ServeHTTP(w, r)
			return
		}
		// Корень кастомного домена → страница статуса (внутренние ссылки уже вида /status/<slug>/...).
		if r.URL.Path == "/" {
			if slug, err := s.resolver.SlugByCustomDomain(r.Context(), hostOnly(r.Host)); err == nil {
				r.URL.Path = "/status/" + slug
			}
		}
		s.ssrProxy.ServeHTTP(w, r)
	})
}

// isAPIPath сообщает, относится ли путь к API (проксируется на api-бэкенд).
func isAPIPath(path string) bool {
	return path == "/api" || strings.HasPrefix(path, "/api/")
}

// hostOnly отбрасывает порт у Host.
func hostOnly(host string) string {
	if i := strings.IndexByte(host, ':'); i >= 0 {
		return host[:i]
	}
	return host
}
