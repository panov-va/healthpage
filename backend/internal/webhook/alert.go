package webhook

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
)

// Alert — нормализованный алерт (общий для Grafana/Prometheus): то, из чего хендлер создаёт или
// закрывает инцидент.
type Alert struct {
	DedupKey string // ключ идемпотентности (один открытый инцидент на ключ)
	Firing   bool   // true=firing (открыть), false=resolved (закрыть)
	Title    string
	Body     string
	Labels   map[string]string // для маппинга на компоненты
}

// alertmanagerPayload — формат webhook'а Alertmanager. Grafana Unified Alerting шлёт
// совместимый payload, поэтому Grafana и Prometheus парсятся одинаково.
type alertmanagerPayload struct {
	Alerts []alertmanagerAlert `json:"alerts"`
}

type alertmanagerAlert struct {
	Status      string            `json:"status"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	Fingerprint string            `json:"fingerprint"`
}

// ParseGrafana разбирает payload Grafana Unified Alerting (Alertmanager-совместимый).
func ParseGrafana(body []byte) ([]Alert, error) { return parseAlertmanager(body) }

// ParsePrometheus разбирает payload Prometheus Alertmanager.
func ParsePrometheus(body []byte) ([]Alert, error) { return parseAlertmanager(body) }

func parseAlertmanager(body []byte) ([]Alert, error) {
	var p alertmanagerPayload
	if err := json.Unmarshal(body, &p); err != nil {
		return nil, err
	}
	if len(p.Alerts) == 0 {
		return nil, errors.New("webhook: payload без alerts")
	}
	out := make([]Alert, 0, len(p.Alerts))
	for _, a := range p.Alerts {
		out = append(out, Alert{
			DedupKey: dedupKey(a),
			Firing:   strings.EqualFold(a.Status, "firing"),
			Title:    alertTitle(a),
			Body:     firstNonEmpty(a.Annotations["description"], a.Annotations["message"], a.Annotations["summary"]),
			Labels:   a.Labels,
		})
	}
	return out, nil
}

// dedupKey берёт fingerprint алерта, а при его отсутствии вычисляет стабильный хэш по меткам.
func dedupKey(a alertmanagerAlert) string {
	if a.Fingerprint != "" {
		return a.Fingerprint
	}
	keys := make([]string, 0, len(a.Labels))
	for k := range a.Labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(a.Labels[k])
		b.WriteByte(',')
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:16])
}

func alertTitle(a alertmanagerAlert) string {
	return firstNonEmpty(a.Annotations["summary"], a.Labels["alertname"], "Инцидент")
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
