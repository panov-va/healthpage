package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/healthpage/backend/internal/domain"
)

// accountPlan возвращает эффективный тариф аккаунта. При ошибке пишет 500 и возвращает ok=false.
func (s *server) accountPlan(w http.ResponseWriter, r *http.Request, accountID uuid.UUID) (domain.BillingPlan, bool) {
	acc, err := s.store.AccountByID(r.Context(), accountID)
	if err != nil {
		writeServerError(w, err)
		return "", false
	}
	return acc.BillingPlan, true
}

// requireFeature проверяет доступность premium-возможности на тарифе. Если недоступна — пишет
// 403 feature_required и возвращает false.
func requireFeature(w http.ResponseWriter, plan domain.BillingPlan, f domain.Feature) bool {
	if domain.PlanAllows(plan, f) {
		return true
	}
	writeError(w, http.StatusForbidden, "feature_required",
		"возможность доступна на тарифе Premium")
	return false
}

// jsonEnablesValue сообщает, что RawMessage-поле ЗАДАЁТ непустое значение (не отсутствует,
// не null, не пустая строка) — т.е. включает фичу. Для объектов (smtp_config) — не null.
func jsonEnablesString(raw json.RawMessage) bool {
	if len(raw) == 0 || string(raw) == "null" {
		return false
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return strings.TrimSpace(s) != ""
	}
	return true // не строка (объект) и не null → задаёт значение
}

// gatePagePremiumFeatures блокирует ВКЛЮЧЕНИЕ premium-фич на тарифе Free (этап 6.7).
// Выключение/очистка (null/false/пусто) не гейтятся. Возвращает false (ответ уже записан),
// если хотя бы одна включаемая фича недоступна на тарифе аккаунта.
func (s *server) gatePagePremiumFeatures(w http.ResponseWriter, r *http.Request, accountID uuid.UUID, req patchPageRequest) bool {
	// Быстрый путь: если ничего платного не включается — план не загружаем.
	enablesDomain := jsonEnablesString(req.CustomDomain)
	enablesPrivate := (req.Visibility != nil && *req.Visibility == string(domain.VisibilityPrivate)) ||
		jsonEnablesString(req.Password)
	enablesSMTP := (req.SMTPConfig != nil && string(req.SMTPConfig) != "null") || jsonEnablesString(req.FromEmail)
	enablesWhiteLabel := req.HidePoweredBy != nil && *req.HidePoweredBy

	if !enablesDomain && !enablesPrivate && !enablesSMTP && !enablesWhiteLabel {
		return true
	}

	plan, ok := s.accountPlan(w, r, accountID)
	if !ok {
		return false
	}
	if enablesDomain && !requireFeature(w, plan, domain.FeatureCustomDomain) {
		return false
	}
	if enablesPrivate && !requireFeature(w, plan, domain.FeaturePrivatePages) {
		return false
	}
	if enablesSMTP && !requireFeature(w, plan, domain.FeatureCustomSMTP) {
		return false
	}
	if enablesWhiteLabel && !requireFeature(w, plan, domain.FeatureWhiteLabel) {
		return false
	}
	return true
}
