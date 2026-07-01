package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/healthpage/backend/internal/domain"
	"github.com/healthpage/backend/internal/store/db"
)

// CreateImportJob создаёт задачу импорта в статусе pending.
func (s *Store) CreateImportJob(ctx context.Context, pageID, accountID uuid.UUID, source domain.ImportSource, region domain.ImportRegion, subdomain string, mode domain.ImportMode) (domain.ImportJob, error) {
	row, err := s.q.CreateImportJob(ctx, db.CreateImportJobParams{
		StatusPageID: pageID,
		AccountID:    accountID,
		Source:       string(source),
		Region:       regionToStr(region),
		Subdomain:    subdomain,
		Mode:         string(mode),
	})
	if err != nil {
		return domain.ImportJob{}, fmt.Errorf("store: create import job: %w", err)
	}
	return mapImportJob(row), nil
}

// ImportJobByID возвращает задачу по id. ErrNotFound если нет.
func (s *Store) ImportJobByID(ctx context.Context, id uuid.UUID) (domain.ImportJob, error) {
	row, err := s.q.GetImportJob(ctx, id)
	if err != nil {
		return domain.ImportJob{}, wrapNotFound(err)
	}
	return mapImportJob(row), nil
}

// UpdateImportJob сохраняет статус/отчёт/ошибку и (опционально) время завершения.
func (s *Store) UpdateImportJob(ctx context.Context, id uuid.UUID, status domain.ImportStatus, report domain.ImportReport, errMsg string, finishedAt *time.Time) (domain.ImportJob, error) {
	rb, err := json.Marshal(report)
	if err != nil {
		return domain.ImportJob{}, fmt.Errorf("store: marshal report: %w", err)
	}
	var errPtr *string
	if errMsg != "" {
		errPtr = &errMsg
	}
	row, err := s.q.UpdateImportJob(ctx, db.UpdateImportJobParams{
		ID:         id,
		Status:     string(status),
		Report:     rb,
		Error:      errPtr,
		FinishedAt: finishedAt,
	})
	if err != nil {
		return domain.ImportJob{}, fmt.Errorf("store: update import job: %w", err)
	}
	return mapImportJob(row), nil
}

// ExternalMapping возвращает internal id по внешнему (идемпотентность импорта). ok=false если нет.
func (s *Store) ExternalMapping(ctx context.Context, pageID uuid.UUID, source domain.ImportSource, entityType, externalID string) (uuid.UUID, bool, error) {
	id, err := s.q.GetExternalMapping(ctx, db.GetExternalMappingParams{
		StatusPageID: pageID,
		Source:       string(source),
		EntityType:   entityType,
		ExternalID:   externalID,
	})
	if err != nil {
		if errors.Is(wrapNotFound(err), ErrNotFound) {
			return uuid.Nil, false, nil
		}
		return uuid.Nil, false, fmt.Errorf("store: get external mapping: %w", err)
	}
	return id, true, nil
}

// SetExternalMapping сохраняет соответствие внешний id → наш internal id.
func (s *Store) SetExternalMapping(ctx context.Context, pageID uuid.UUID, source domain.ImportSource, entityType, externalID string, internalID uuid.UUID) error {
	if err := s.q.UpsertExternalMapping(ctx, db.UpsertExternalMappingParams{
		StatusPageID: pageID,
		Source:       string(source),
		EntityType:   entityType,
		ExternalID:   externalID,
		InternalID:   internalID,
	}); err != nil {
		return fmt.Errorf("store: upsert external mapping: %w", err)
	}
	return nil
}

func mapImportJob(j db.ImportJob) domain.ImportJob {
	var report domain.ImportReport
	if len(j.Report) > 0 {
		_ = json.Unmarshal(j.Report, &report)
	}
	region := domain.ImportRegion("")
	if j.Region != nil {
		region = domain.ImportRegion(*j.Region)
	}
	errMsg := ""
	if j.Error != nil {
		errMsg = *j.Error
	}
	return domain.ImportJob{
		ID:           j.ID,
		StatusPageID: j.StatusPageID,
		AccountID:    j.AccountID,
		Source:       domain.ImportSource(j.Source),
		Region:       region,
		Subdomain:    j.Subdomain,
		Mode:         domain.ImportMode(j.Mode),
		Status:       domain.ImportStatus(j.Status),
		Report:       report,
		Error:        errMsg,
		CreatedAt:    j.CreatedAt,
		FinishedAt:   j.FinishedAt,
		UpdatedAt:    j.UpdatedAt,
	}
}

func regionToStr(r domain.ImportRegion) *string {
	if r == "" {
		return nil
	}
	v := string(r)
	return &v
}
