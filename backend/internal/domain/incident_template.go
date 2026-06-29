package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// IncidentTemplate — преднастроенная заготовка инцидента для типовых ситуаций (DESIGN §3.3, §5):
// шаблон заголовка/тела первого обновления, impact по умолчанию и преднастроенные затронутые
// компоненты. Шаблон сам по себе не влияет на статус компонентов — это лишь заготовка, которую
// оператор применяет при создании инцидента (применение — на стороне клиента, MVP).
type IncidentTemplate struct {
	ID            uuid.UUID
	StatusPageID  uuid.UUID
	Name          string
	TitleTmpl     string
	BodyTmpl      string
	DefaultImpact IncidentImpact
	CreatedAt     time.Time
	UpdatedAt     time.Time

	// Связь (заполняется store при чтении агрегата). Переиспользуем IncidentComponent:
	// пары {component_id, статус-в-инциденте} 1:1 с тем, что уходит в IncidentCreate.
	DefaultComponents []IncidentComponent
}

// ErrInvalidTemplateName — у шаблона должно быть непустое имя (оператор отличает шаблоны по нему).
var ErrInvalidTemplateName = errors.New("incident template name must not be empty")

// Validate проверяет инварианты шаблона: непустое имя и допустимый impact. Заготовки текста
// (TitleTmpl/BodyTmpl) и набор компонентов могут быть пустыми.
func (t IncidentTemplate) Validate() error {
	if t.Name == "" {
		return ErrInvalidTemplateName
	}
	if !t.DefaultImpact.IsValid() {
		return ErrInvalidIncidentImpact
	}
	return nil
}
