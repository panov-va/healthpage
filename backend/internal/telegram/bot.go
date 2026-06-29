package telegram

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/healthpage/backend/internal/domain"
	"github.com/healthpage/backend/internal/store"
)

// BotStore — то, что боту нужно от хранилища для управления подпиской.
type BotStore interface {
	StatusPageBySlug(ctx context.Context, slug string) (domain.StatusPage, error)
	SubscriberByPageChannelAddress(ctx context.Context, pageID uuid.UUID, channel domain.SubscriberChannel, address string) (domain.Subscriber, error)
	SubscribersByChannelAddress(ctx context.Context, channel domain.SubscriberChannel, address string) ([]domain.Subscriber, error)
	StatusPageByID(ctx context.Context, id uuid.UUID) (domain.StatusPage, error)
	CreateSubscriber(ctx context.Context, sub domain.Subscriber) (domain.Subscriber, error)
	DeleteSubscriber(ctx context.Context, id uuid.UUID) error
}

// updateAPI — подмножество Bot API, нужное боту (long polling + ответ). *Client его реализует.
type updateAPI interface {
	GetUpdates(ctx context.Context, offset int64, timeout int) ([]Update, error)
	SendMessage(ctx context.Context, chatID int64, text string) error
}

// Bot управляет подпиской через команды бота (DESIGN §3.4): long-poll getUpdates, обработка
// /start <slug> (подписка на страницу), /stop [slug] (отписка). Подписка в Telegram подтверждена
// сразу (старт бота = явное согласие), double opt-in не нужен.
type Bot struct {
	api         updateAPI
	store       BotStore
	pollTimeout int // секунды long polling
	log         *slog.Logger
}

// NewBot собирает бота. pollTimeout<=0 → 30с. logger=nil → slog.Default().
func NewBot(api updateAPI, st BotStore, pollTimeout int, logger *slog.Logger) *Bot {
	if pollTimeout <= 0 {
		pollTimeout = 30
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Bot{api: api, store: st, pollTimeout: pollTimeout, log: logger}
}

// Run крутит цикл long polling до отмены ctx. Ошибки getUpdates логируются с паузой и повтором.
func (b *Bot) Run(ctx context.Context) error {
	var offset int64
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		updates, err := b.api.GetUpdates(ctx, offset, b.pollTimeout)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			b.log.Error("telegram bot: getUpdates", "err", err)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(retryDelay(err)):
			}
			continue
		}
		for _, u := range updates {
			offset = u.UpdateID + 1
			if u.Message == nil || strings.TrimSpace(u.Message.Text) == "" {
				continue
			}
			b.handleMessage(ctx, *u.Message)
		}
	}
}

// retryDelay выбирает паузу перед повтором getUpdates (учитывает 429 retry_after).
func retryDelay(err error) time.Duration {
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.RetryAfter > 0 {
		return apiErr.RetryAfter
	}
	return 3 * time.Second
}

// handleMessage разбирает команду и направляет в обработчик.
func (b *Bot) handleMessage(ctx context.Context, m Message) {
	cmd, arg := parseCommand(m.Text)
	locale := ""
	if m.From != nil {
		locale = m.From.LanguageCode
	}
	switch cmd {
	case "/start":
		b.handleStart(ctx, m, arg, locale)
	case "/stop":
		b.handleStop(ctx, m, arg, locale)
	default:
		b.reply(ctx, m.Chat.ID, dict(locale).help)
	}
}

// handleStart подписывает чат на страницу из deep-link payload (slug). Идемпотентно.
func (b *Bot) handleStart(ctx context.Context, m Message, slug, locale string) {
	if slug == "" {
		b.reply(ctx, m.Chat.ID, dict(locale).startNoArg)
		return
	}
	page, err := b.store.StatusPageBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			b.reply(ctx, m.Chat.ID, dict(locale).pageNotFound)
			return
		}
		b.log.Error("telegram bot: load page by slug", "slug", slug, "err", err)
		return
	}
	// Ответы — на локали страницы (точнее, чем язык клиента Telegram).
	t := dict(page.DefaultLocale)
	addr := strconv.FormatInt(m.Chat.ID, 10)

	_, err = b.store.SubscriberByPageChannelAddress(ctx, page.ID, domain.ChannelTelegram, addr)
	switch {
	case err == nil:
		b.reply(ctx, m.Chat.ID, t.already(page.Name))
		return
	case !errors.Is(err, store.ErrNotFound):
		b.log.Error("telegram bot: lookup subscriber", "page", page.ID, "err", err)
		return
	}

	if _, err := b.store.CreateSubscriber(ctx, domain.Subscriber{
		StatusPageID: page.ID,
		Channel:      domain.ChannelTelegram,
		Address:      addr,
		Confirmed:    true, // старт бота = явное согласие, double opt-in не нужен
		Scope:        domain.ScopePage,
	}); err != nil {
		b.log.Error("telegram bot: create subscriber", "page", page.ID, "err", err)
		return
	}
	b.log.Info("telegram bot: subscribed", "page", page.ID, "chat", m.Chat.ID)
	b.reply(ctx, m.Chat.ID, t.subscribed(page.Name))
}

// handleStop отписывает чат: /stop <slug> — от одной страницы, /stop — от всех подписок чата.
func (b *Bot) handleStop(ctx context.Context, m Message, slug, locale string) {
	addr := strconv.FormatInt(m.Chat.ID, 10)

	if slug != "" {
		page, err := b.store.StatusPageBySlug(ctx, slug)
		if err != nil {
			// Нет такой страницы → подписки на неё быть не может.
			b.reply(ctx, m.Chat.ID, dict(locale).notSubscribed)
			return
		}
		t := dict(page.DefaultLocale)
		sub, err := b.store.SubscriberByPageChannelAddress(ctx, page.ID, domain.ChannelTelegram, addr)
		if errors.Is(err, store.ErrNotFound) {
			b.reply(ctx, m.Chat.ID, t.notSubscribed)
			return
		}
		if err != nil {
			b.log.Error("telegram bot: lookup subscriber", "page", page.ID, "err", err)
			return
		}
		if err := b.store.DeleteSubscriber(ctx, sub.ID); err != nil {
			b.log.Error("telegram bot: delete subscriber", "id", sub.ID, "err", err)
			return
		}
		b.reply(ctx, m.Chat.ID, t.stoppedOne(page.Name))
		return
	}

	subs, err := b.store.SubscribersByChannelAddress(ctx, domain.ChannelTelegram, addr)
	if err != nil {
		b.log.Error("telegram bot: list subscriptions", "chat", m.Chat.ID, "err", err)
		return
	}
	if len(subs) == 0 {
		b.reply(ctx, m.Chat.ID, dict(locale).notSubscribed)
		return
	}
	removed := 0
	for _, sub := range subs {
		if err := b.store.DeleteSubscriber(ctx, sub.ID); err != nil {
			b.log.Error("telegram bot: delete subscriber", "id", sub.ID, "err", err)
			continue
		}
		removed++
	}
	b.log.Info("telegram bot: unsubscribed all", "chat", m.Chat.ID, "removed", removed)
	b.reply(ctx, m.Chat.ID, dict(locale).stoppedAll(removed))
}

// reply отправляет ответ, логируя ошибку (не прерывает обработку остальных сообщений).
func (b *Bot) reply(ctx context.Context, chatID int64, text string) {
	if err := b.api.SendMessage(ctx, chatID, text); err != nil {
		b.log.Warn("telegram bot: reply failed", "chat", chatID, "err", err)
	}
}

// parseCommand извлекает команду (в нижнем регистре, без @botname) и аргумент.
// "/start@MyBot acme" → ("/start", "acme"); "  /STOP  " → ("/stop", "").
func parseCommand(text string) (cmd, arg string) {
	text = strings.TrimSpace(text)
	first, rest, _ := strings.Cut(text, " ")
	if at := strings.IndexByte(first, '@'); at >= 0 {
		first = first[:at]
	}
	return strings.ToLower(first), strings.TrimSpace(rest)
}
