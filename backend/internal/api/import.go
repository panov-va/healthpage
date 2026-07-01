package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/healthpage/backend/internal/domain"
	"github.com/healthpage/backend/internal/importer"
	"github.com/healthpage/backend/internal/store"
)

// ImportPublisher публикует задачу импорта в очередь (реализуется queue.Publisher).
type ImportPublisher interface {
	PublishImport(ctx context.Context, body []byte) error
}

// ── DTO импорта (синхронно с openapi ImportRequest / ImportJob) ──

type importRequest struct {
	Source       string `json:"source"`
	StatusPageID string `json:"status_page_id"`
	Region       string `json:"region"`
	APIKey       string `json:"api_key"`
	Subdomain    string `json:"subdomain"`
	Mode         string `json:"mode"`
}

type importJobResponse struct {
	ID         string              `json:"id"`
	Source     string              `json:"source"`
	Region     *string             `json:"region"`
	Status     string              `json:"status"`
	Report     domain.ImportReport `json:"report"`
	CreatedAt  string              `json:"created_at"`
	FinishedAt *string             `json:"finished_at"`
}

func toImportJobResponse(j domain.ImportJob) importJobResponse {
	out := importJobResponse{
		ID:         j.ID.String(),
		Source:     string(j.Source),
		Status:     string(j.Status),
		Report:     j.Report,
		CreatedAt:  j.CreatedAt.UTC().Format(time.RFC3339),
		FinishedAt: rfc3339Ptr(j.FinishedAt),
	}
	if j.Region != "" {
		v := string(j.Region)
		out.Region = &v
	}
	return out
}

var slugSanitize = regexp.MustCompile(`[^a-z0-9]+`)

// handleStartImport создаёт задачу импорта и ставит её в очередь. Только оператор.
func (s *server) handleStartImport(w http.ResponseWriter, r *http.Request) {
	if s.importPublisher == nil {
		writeError(w, http.StatusServiceUnavailable, "import_disabled", "импорт не настроен (нет очереди)")
		return
	}
	acc, ok := s.operatorAccount(w, r)
	if !ok {
		return
	}
	var req importRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	source := domain.ImportSource(req.Source)
	if !source.IsValid() {
		writeError(w, http.StatusUnprocessableEntity, "invalid_request", "недопустимый source")
		return
	}
	// MVP: реализован только StatusPal (остальные адаптеры — позже).
	if source != domain.SourceStatusPal {
		writeError(w, http.StatusUnprocessableEntity, "unsupported_source", "пока поддерживается только импорт из statuspal")
		return
	}
	if strings.TrimSpace(req.APIKey) == "" || strings.TrimSpace(req.Subdomain) == "" {
		writeError(w, http.StatusUnprocessableEntity, "invalid_request", "api_key и subdomain обязательны")
		return
	}
	region := domain.ImportRegion(req.Region)
	if !region.IsValid() {
		writeError(w, http.StatusUnprocessableEntity, "invalid_request", "недопустимый region (us|eu)")
		return
	}
	mode := domain.ImportMode(req.Mode)
	if req.Mode == "" {
		mode = domain.ModeSkip
	}
	if !mode.IsValid() {
		writeError(w, http.StatusUnprocessableEntity, "invalid_request", "недопустимый mode (skip|update)")
		return
	}

	// Целевая страница: заданная (проверяем владение) или новая из subdomain.
	page, ok := s.resolveImportTargetPage(w, r, acc, req.StatusPageID, source, req.Subdomain)
	if !ok {
		return
	}

	job, err := s.store.CreateImportJob(r.Context(), page.ID, acc.ID, source, region, req.Subdomain, mode)
	if err != nil {
		writeServerError(w, err)
		return
	}

	// Публикуем задачу с api_key (в БД ключ не хранится — «не дольше задачи», §4.3).
	msg, err := importer.Message{
		JobID:     job.ID.String(),
		Source:    string(source),
		Region:    string(region),
		Subdomain: req.Subdomain,
		Mode:      string(mode),
		APIKey:    req.APIKey,
	}.Marshal()
	if err != nil {
		writeServerError(w, err)
		return
	}
	if err := s.importPublisher.PublishImport(r.Context(), msg); err != nil {
		// Задача создана, но не поставлена — помечаем failed, чтобы не «зависла» в pending.
		now := time.Now().UTC()
		_, _ = s.store.UpdateImportJob(r.Context(), job.ID, domain.ImportFailed, job.Report, "не удалось поставить задачу в очередь", &now)
		writeServerError(w, err)
		return
	}

	writeJSON(w, http.StatusAccepted, toImportJobResponse(job))
}

// resolveImportTargetPage возвращает целевую страницу импорта: если задан status_page_id —
// проверяет владение; иначе создаёт новую страницу с уникальным slug из subdomain.
func (s *server) resolveImportTargetPage(w http.ResponseWriter, r *http.Request, acc domain.Account, rawPageID string, source domain.ImportSource, subdomain string) (domain.StatusPage, bool) {
	if rawPageID != "" {
		id, err := uuid.Parse(rawPageID)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "invalid_request", "некорректный status_page_id")
			return domain.StatusPage{}, false
		}
		return s.authorizePage(w, r, id)
	}

	base := slugSanitize.ReplaceAllString(strings.ToLower(subdomain), "-")
	base = strings.Trim(base, "-")
	if base == "" {
		base = string(source)
	}
	name := subdomain
	// Пытаемся создать с уникальным slug (несколько попыток при коллизии).
	for attempt := 0; attempt < 5; attempt++ {
		slug := base
		if attempt > 0 {
			slug = fmt.Sprintf("%s-%d", base, attempt+1)
		}
		page, err := s.store.CreateStatusPage(r.Context(), acc.ID, acc.OwnerUserID, name, "", slug, "UTC", "ru", string(domain.VisibilityPublic))
		if err == nil {
			return page, true
		}
		if !errors.Is(err, store.ErrSlugTaken) {
			writeServerError(w, err)
			return domain.StatusPage{}, false
		}
	}
	writeError(w, http.StatusConflict, "slug_taken", "не удалось подобрать slug для новой страницы; укажите status_page_id")
	return domain.StatusPage{}, false
}

// handleGetImportJob возвращает статус и отчёт задачи импорта (только владелец аккаунта).
func (s *server) handleGetImportJob(w http.ResponseWriter, r *http.Request) {
	acc, ok := s.operatorAccount(w, r)
	if !ok {
		return
	}
	id, ok := pathUUID(w, r, "job_id")
	if !ok {
		return
	}
	job, err := s.store.ImportJobByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "задача не найдена")
			return
		}
		writeServerError(w, err)
		return
	}
	if job.AccountID != acc.ID {
		writeError(w, http.StatusNotFound, "not_found", "задача не найдена")
		return
	}
	writeJSON(w, http.StatusOK, toImportJobResponse(job))
}
