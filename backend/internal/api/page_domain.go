package api

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/healthpage/backend/internal/store"
)

type domainStatusResponse struct {
	CustomDomain   *string `json:"custom_domain"`
	DomainVerified bool    `json:"domain_verified"`
	CNAMETarget    string  `json:"cname_target"`
}

// handleVerifyDomain проверяет, что CNAME собственного домена страницы (custom_domain) указывает
// на наш целевой хост (cnameTarget), и фиксирует domain_verified (этап 4.3). Возвращает текущий
// статус с целевым хостом для инструкции оператору. Домен не задан → 422.
func (s *server) handleVerifyDomain(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r, "page")
	if !ok {
		return
	}
	page, ok := s.authorizePage(w, r, id)
	if !ok {
		return
	}
	if page.CustomDomain == nil || *page.CustomDomain == "" {
		writeError(w, http.StatusUnprocessableEntity, "no_domain", "собственный домен не задан")
		return
	}

	verified := s.cnameMatchesTarget(r, *page.CustomDomain)
	if verified != page.DomainVerified {
		if err := s.store.SetDomainVerified(r.Context(), page.ID, verified); err != nil {
			writeServerError(w, err)
			return
		}
	}

	// Подключение в Dokploy (замена edge/tls-manager в проде, DEPLOY.md): при первой успешной
	// верификации связываем домен с приложением public-ssr — дальше Traefik/Let's Encrypt
	// обслуживает его сам Dokploy. dokploy=nil — интеграция не настроена, домен остаётся только
	// verified. DokployDomainID уже задан — домен уже подключён, повторно не создаём (idempotent).
	if verified && s.dokploy != nil && page.DokployDomainID == nil {
		domainID, err := s.dokploy.CreateDomain(r.Context(), *page.CustomDomain)
		if err != nil {
			log.Printf("dokploy: create domain %q (page %s): %v", *page.CustomDomain, page.ID, err)
			writeError(w, http.StatusBadGateway, "dokploy_error", "домен верифицирован, но не удалось подключить его в инфраструктуре — попробуйте ещё раз")
			return
		}
		if err := s.store.SetDokployDomainID(r.Context(), page.ID, &domainID); err != nil {
			writeServerError(w, err)
			return
		}
	}

	writeJSON(w, http.StatusOK, domainStatusResponse{
		CustomDomain:   page.CustomDomain,
		DomainVerified: verified,
		CNAMETarget:    s.cnameTarget,
	})
}

// cnameMatchesTarget резолвит CNAME домена и сравнивает каноническое имя с cnameTarget
// (без учёта регистра и завершающей точки). Ошибка резолва / пустой target → не верифицировано.
func (s *server) cnameMatchesTarget(r *http.Request, host string) bool {
	if s.cnameTarget == "" {
		return false
	}
	cname, err := s.cnameResolver(r.Context(), host)
	if err != nil {
		return false
	}
	return normalizeHost(cname) == normalizeHost(s.cnameTarget)
}

func normalizeHost(h string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(h)), ".")
}

type slugByDomainResponse struct {
	Slug string `json:"slug"`
}

// handleGetSlugByDomain резолвит slug страницы по верифицированному собственному домену клиента
// (этап 4.3) — используется public-ssr для маршрутизации по Host-заголовку, чтобы корень
// кастомного домена открывал страницу статуса, а не лендинг.
func (s *server) handleGetSlugByDomain(w http.ResponseWriter, r *http.Request) {
	host := normalizeHost(r.URL.Query().Get("domain"))
	if host == "" {
		writeError(w, http.StatusNotFound, "not_found", "домен не найден")
		return
	}
	slug, err := s.store.SlugByCustomDomain(r.Context(), host)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "домен не найден")
		} else {
			writeServerError(w, err)
		}
		return
	}
	writeJSON(w, http.StatusOK, slugByDomainResponse{Slug: slug})
}
