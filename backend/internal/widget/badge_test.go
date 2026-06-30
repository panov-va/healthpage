package widget

import (
	"strings"
	"testing"
)

func TestBuildBadge(t *testing.T) {
	svg := string(BuildBadge("operational", "en"))
	if !strings.HasPrefix(svg, "<svg") || !strings.Contains(svg, "operational") {
		t.Fatalf("svg: %s", svg)
	}
	if !strings.Contains(svg, "#1a7f37") {
		t.Fatal("ожидался зелёный цвет для operational")
	}
	// RU + неизвестный статус → подставляется как есть, дефолтный цвет
	ru := string(BuildBadge("major_outage", "ru"))
	if !strings.Contains(ru, "сбой") || !strings.Contains(ru, "#cf222e") {
		t.Fatalf("ru svg: %s", ru)
	}
	if !strings.Contains(ru, "статус") {
		t.Fatal("ожидался label 'статус'")
	}
}
