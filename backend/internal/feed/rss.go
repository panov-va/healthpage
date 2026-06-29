// Package feed — публичные фиды страницы статуса (этап 3.6): RSS 2.0 (инциденты + работы) и
// iCal (плановые работы). Чистые сериализаторы поверх доменных сущностей; без БД/HTTP.
package feed

import (
	"encoding/xml"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/healthpage/backend/internal/domain"
)

// maxItems ограничивает размер фида (последние записи).
const maxItems = 50

type rss struct {
	XMLName xml.Name   `xml:"rss"`
	Version string     `xml:"version,attr"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title       string    `xml:"title"`
	Link        string    `xml:"link"`
	Description string    `xml:"description"`
	Language    string    `xml:"language,omitempty"`
	Items       []rssItem `xml:"item"`
}

type rssItem struct {
	Title       string  `xml:"title"`
	Link        string  `xml:"link"`
	Description string  `xml:"description"`
	PubDate     string  `xml:"pubDate"`
	GUID        rssGUID `xml:"guid"`
}

type rssGUID struct {
	Value       string `xml:",chardata"`
	IsPermaLink bool   `xml:"isPermaLink,attr"`
}

// feedItem — промежуточное представление для сортировки разнородных записей по дате.
type feedItem struct {
	item rssItem
	at   time.Time
}

// BuildRSS собирает RSS 2.0 фид инцидентов и работ страницы (последние maxItems по дате).
func BuildRSS(page domain.StatusPage, incidents []domain.Incident, maintenances []domain.Maintenance, baseURL string) ([]byte, error) {
	pageURL := pageURL(baseURL, page.Slug)

	items := make([]feedItem, 0, len(incidents)+len(maintenances))
	for _, inc := range incidents {
		at := inc.StartedAt
		desc := string(inc.CurrentStatus)
		if u, ok := latestUpdate(inc.Updates); ok {
			at = u.CreatedAt
			if u.Body != "" {
				desc = u.Body
			}
		}
		items = append(items, feedItem{at: at, item: rssItem{
			Title:       fmt.Sprintf("%s (%s)", inc.Title, inc.Impact),
			Link:        pageURL + "/incidents/" + inc.ID.String(),
			Description: desc,
			PubDate:     at.UTC().Format(time.RFC1123Z),
			GUID:        rssGUID{Value: "incident:" + inc.ID.String(), IsPermaLink: false},
		}})
	}
	for _, m := range maintenances {
		desc := fmt.Sprintf("%s — %s", m.ScheduledStart.UTC().Format(time.RFC1123Z), m.ScheduledEnd.UTC().Format(time.RFC1123Z))
		if m.Description != "" {
			desc = m.Description + "\n" + desc
		}
		items = append(items, feedItem{at: m.ScheduledStart, item: rssItem{
			Title:       fmt.Sprintf("%s (%s)", m.Title, m.Status),
			Link:        pageURL + "/maintenances",
			Description: desc,
			PubDate:     m.ScheduledStart.UTC().Format(time.RFC1123Z),
			GUID:        rssGUID{Value: "maintenance:" + m.ID.String(), IsPermaLink: false},
		}})
	}

	sort.SliceStable(items, func(i, j int) bool { return items[i].at.After(items[j].at) })
	if len(items) > maxItems {
		items = items[:maxItems]
	}

	ch := rssChannel{
		Title:       page.Name + " — status",
		Link:        pageURL,
		Description: feedDescription(page),
		Language:    page.DefaultLocale,
		Items:       make([]rssItem, len(items)),
	}
	for i, it := range items {
		ch.Items[i] = it.item
	}

	out, err := xml.MarshalIndent(rss{Version: "2.0", Channel: ch}, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("feed: marshal rss: %w", err)
	}
	return append([]byte(xml.Header), out...), nil
}

func feedDescription(page domain.StatusPage) string {
	if strings.HasPrefix(strings.ToLower(page.DefaultLocale), "en") {
		return "Incidents and scheduled maintenance for " + page.Name
	}
	return "Инциденты и плановые работы — " + page.Name
}

// latestUpdate возвращает обновление с максимальным CreatedAt.
func latestUpdate(updates []domain.IncidentUpdate) (domain.IncidentUpdate, bool) {
	if len(updates) == 0 {
		return domain.IncidentUpdate{}, false
	}
	latest := updates[0]
	for _, u := range updates[1:] {
		if u.CreatedAt.After(latest.CreatedAt) {
			latest = u
		}
	}
	return latest, true
}

func pageURL(baseURL, slug string) string {
	return strings.TrimRight(baseURL, "/") + "/status/" + slug
}
