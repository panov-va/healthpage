// Package widget строит встраиваемый SVG-бейдж статуса страницы (этап 4.6).
package widget

import (
	"fmt"
	"html"
	"strings"
	"unicode/utf8"
)

// цвета статусов — согласованы с публичной страницей (lib/badge / globals.css).
var statusColor = map[string]string{
	"operational":          "#1a7f37",
	"degraded_performance": "#bf8700",
	"under_maintenance":    "#0969da",
	"partial_outage":       "#d4670a",
	"major_outage":         "#cf222e",
}

var statusLabel = map[string]map[string]string{
	"ru": {
		"operational":          "работает",
		"degraded_performance": "снижение",
		"under_maintenance":    "тех. работы",
		"partial_outage":       "частичный сбой",
		"major_outage":         "сбой",
	},
	"en": {
		"operational":          "operational",
		"degraded_performance": "degraded",
		"under_maintenance":    "maintenance",
		"partial_outage":       "partial outage",
		"major_outage":         "major outage",
	},
}

// BuildBadge возвращает SVG-бейдж «<label>: <статус>» (shields-стиль, две секции).
// overall — общий статус страницы; lang — ru|en (дефолт ru).
func BuildBadge(overall, lang string) []byte {
	if lang != "en" {
		lang = "ru"
	}
	label := "статус"
	if lang == "en" {
		label = "status"
	}
	value := statusLabel[lang][overall]
	if value == "" {
		value = overall
	}
	color := statusColor[overall]
	if color == "" {
		color = "#555"
	}

	lw := segWidth(label)
	vw := segWidth(value)
	total := lw + vw
	aria := html.EscapeString(label + ": " + value)

	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="20" role="img" aria-label="%s">`, total, aria)
	fmt.Fprintf(&b, `<rect width="%d" height="20" rx="3" fill="#555"/>`, lw)
	fmt.Fprintf(&b, `<rect x="%d" width="%d" height="20" rx="3" fill="%s"/>`, lw, vw, color)
	// перекрываем стык, чтобы скругление не оставляло зазор
	fmt.Fprintf(&b, `<rect x="%d" width="6" height="20" fill="%s"/>`, lw-3, color)
	b.WriteString(`<g fill="#fff" text-anchor="middle" font-family="Verdana,DejaVu Sans,sans-serif" font-size="11">`)
	fmt.Fprintf(&b, `<text x="%d" y="14">%s</text>`, lw/2, html.EscapeString(label))
	fmt.Fprintf(&b, `<text x="%d" y="14">%s</text>`, lw+vw/2, html.EscapeString(value))
	b.WriteString(`</g></svg>`)
	return []byte(b.String())
}

// segWidth оценивает ширину секции: ~7px на символ + поля. Достаточно для аккуратного бейджа
// без точных метрик шрифта (кириллица/латиница считаются по рунам).
func segWidth(text string) int {
	w := utf8.RuneCountInString(text)*7 + 14
	if w < 40 {
		w = 40
	}
	return w
}
