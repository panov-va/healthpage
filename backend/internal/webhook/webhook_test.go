package webhook

import (
	"testing"

	"github.com/google/uuid"

	"github.com/healthpage/backend/internal/domain"
)

func TestVerifySignature(t *testing.T) {
	secret := "whsec_test"
	body := []byte(`{"alerts":[]}`)
	sig := Sign(secret, body)

	if !VerifySignature(secret, body, sig) {
		t.Error("валидная подпись должна пройти")
	}
	if !VerifySignature(secret, body, "sha256="+sig) {
		t.Error("префикс sha256= должен поддерживаться")
	}
	if VerifySignature(secret, body, "deadbeef") {
		t.Error("неверная подпись не должна пройти")
	}
	if VerifySignature("other", body, sig) {
		t.Error("другой секрет не должен пройти")
	}
	if VerifySignature(secret, body, "") || VerifySignature("", body, sig) {
		t.Error("пустой секрет/подпись → false")
	}
}

func TestParseGrafanaPrometheus(t *testing.T) {
	body := []byte(`{
		"alerts": [
			{"status":"firing","fingerprint":"abc123","labels":{"alertname":"HighLatency","service":"api"},
			 "annotations":{"summary":"Высокая задержка","description":"p99 > 1s"}},
			{"status":"resolved","fingerprint":"def456","labels":{"alertname":"DiskFull"},"annotations":{}}
		]
	}`)
	for _, parse := range []func([]byte) ([]Alert, error){ParseGrafana, ParsePrometheus} {
		alerts, err := parse(body)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if len(alerts) != 2 {
			t.Fatalf("ожидалось 2 алерта, got %d", len(alerts))
		}
		a0 := alerts[0]
		if !a0.Firing || a0.DedupKey != "abc123" || a0.Title != "Высокая задержка" || a0.Body != "p99 > 1s" {
			t.Errorf("alert[0] неверный: %+v", a0)
		}
		if a0.Labels["service"] != "api" {
			t.Errorf("метки не разобраны: %+v", a0.Labels)
		}
		if alerts[1].Firing {
			t.Error("alert[1] должен быть resolved")
		}
		// Title fallback на alertname.
		if alerts[1].Title != "DiskFull" {
			t.Errorf("alert[1] title fallback: %q", alerts[1].Title)
		}
	}
}

func TestParseErrors(t *testing.T) {
	if _, err := ParseGrafana([]byte(`not json`)); err == nil {
		t.Error("битый JSON → ошибка")
	}
	if _, err := ParseGrafana([]byte(`{"alerts":[]}`)); err == nil {
		t.Error("пустые alerts → ошибка")
	}
}

func TestDedupKeyFallback(t *testing.T) {
	// Без fingerprint ключ вычисляется стабильно по меткам.
	body := []byte(`{"alerts":[{"status":"firing","labels":{"a":"1","b":"2"},"annotations":{}}]}`)
	a1, _ := ParseGrafana(body)
	a2, _ := ParseGrafana(body)
	if a1[0].DedupKey == "" || a1[0].DedupKey != a2[0].DedupKey {
		t.Errorf("dedup-ключ по меткам должен быть стабилен: %q vs %q", a1[0].DedupKey, a2[0].DedupKey)
	}
}

func TestMappingResolve(t *testing.T) {
	cid := uuid.New()
	def := uuid.New()
	m := Mapping{
		Map:                 map[string]string{"api": cid.String()},
		DefaultComponentIDs: []string{def.String()},
		MatchLabel:          "service",
		DefaultImpact:       "critical",
	}
	if m.Impact() != domain.ImpactCritical {
		t.Errorf("impact: want critical, got %s", m.Impact())
	}
	// Совпадение по метке.
	got := m.Resolve(Alert{Labels: map[string]string{"service": "api"}})
	if len(got) != 1 || got[0] != cid {
		t.Errorf("маппинг по метке: %v", got)
	}
	// Нет совпадения → default.
	got = m.Resolve(Alert{Labels: map[string]string{"service": "db"}})
	if len(got) != 1 || got[0] != def {
		t.Errorf("default: %v", got)
	}

	// Пустой маппинг → impact major по умолчанию, без компонентов.
	empty := Mapping{}
	if empty.Impact() != domain.ImpactMajor {
		t.Errorf("пустой impact: want major, got %s", empty.Impact())
	}
	if got := empty.Resolve(Alert{}); len(got) != 0 {
		t.Errorf("пустой маппинг → нет компонентов, got %v", got)
	}
}

func TestParseMapping(t *testing.T) {
	if m, err := ParseMapping(nil); err != nil || m.MatchLabel != "" {
		t.Errorf("nil → пустой маппинг: %v %+v", err, m)
	}
	if _, err := ParseMapping([]byte(`{bad`)); err == nil {
		t.Error("битый JSON → ошибка")
	}
}
