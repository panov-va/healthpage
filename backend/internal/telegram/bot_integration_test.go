package telegram

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/healthpage/backend/internal/domain"
	"github.com/healthpage/backend/internal/store"
)

// TestBotSubscriptionIntegration проверяет флоу подписки бота против реального PostgreSQL:
// /start подписывает (confirmed, scope=page), повтор идемпотентен, /stop отписывает, а новый
// store-запрос SubscribersByChannelAddress находит подписки чата по нескольким страницам.
// Запуск: HEALTHPAGE_TEST_DB=... go test ./internal/telegram/ -run TestBotSubscriptionIntegration
func TestBotSubscriptionIntegration(t *testing.T) {
	dsn := os.Getenv("HEALTHPAGE_TEST_DB")
	if dsn == "" {
		t.Skip("HEALTHPAGE_TEST_DB not set; skipping integration test")
	}
	ctx := context.Background()

	st, err := store.New(ctx, dsn)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	defer st.Close()

	raw, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("raw pool: %v", err)
	}
	defer raw.Close()

	email := "tg-" + uuid.NewString() + "@example.test"
	user, account, err := st.CreateUserWithAccount(ctx, email, "hash", "TG", "TG acc", "ru")
	if err != nil {
		t.Fatalf("CreateUserWithAccount: %v", err)
	}
	t.Cleanup(func() {
		_, _ = raw.Exec(ctx, "DELETE FROM accounts WHERE id=$1", account.ID)
		_, _ = raw.Exec(ctx, "DELETE FROM users WHERE id=$1", user.ID)
	})

	slugA := "tg-" + uuid.NewString()[:8]
	slugB := "tg-" + uuid.NewString()[:8]
	pageA, err := st.CreateStatusPage(ctx, account.ID, user.ID, "Demo A", "", slugA, "UTC", "ru", "public")
	if err != nil {
		t.Fatalf("CreateStatusPage A: %v", err)
	}
	pageB, err := st.CreateStatusPage(ctx, account.ID, user.ID, "Demo B", "", slugB, "UTC", "ru", "public")
	if err != nil {
		t.Fatalf("CreateStatusPage B: %v", err)
	}

	const chatID = int64(424242)
	const addr = "424242"
	bot := NewBot(&fakeAPI{}, st, 0, nil)
	send := func(text string) { bot.handleMessage(ctx, msg(chatID, text)) }

	// /start <slugA> → подписка создана, подтверждена, scope=page.
	send("/start " + slugA)
	sub, err := st.SubscriberByPageChannelAddress(ctx, pageA.ID, domain.ChannelTelegram, addr)
	if err != nil {
		t.Fatalf("подписчик не создан: %v", err)
	}
	if !sub.Confirmed || sub.Scope != domain.ScopePage {
		t.Fatalf("ожидался confirmed page-подписчик: %+v", sub)
	}

	// Повтор /start — без дубликата (идемпотентность по уникальной тройке page/channel/address).
	send("/start " + slugA)
	if _, err := st.SubscriberByPageChannelAddress(ctx, pageA.ID, domain.ChannelTelegram, addr); err != nil {
		t.Fatalf("повтор не должен ломать подписку: %v", err)
	}

	// Подписка на вторую страницу; SubscribersByChannelAddress видит обе.
	send("/start " + slugB)
	all, err := st.SubscribersByChannelAddress(ctx, domain.ChannelTelegram, addr)
	if err != nil {
		t.Fatalf("SubscribersByChannelAddress: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("ожидалось 2 подписки чата, got %d", len(all))
	}

	// /stop <slugA> — снимает только одну.
	send("/stop " + slugA)
	if _, err := st.SubscriberByPageChannelAddress(ctx, pageA.ID, domain.ChannelTelegram, addr); err == nil {
		t.Fatal("подписка на A должна быть удалена")
	}
	if _, err := st.SubscriberByPageChannelAddress(ctx, pageB.ID, domain.ChannelTelegram, addr); err != nil {
		t.Fatalf("подписка на B должна остаться: %v", err)
	}

	// /stop без аргумента — снимает всё оставшееся.
	send("/stop")
	left, err := st.SubscribersByChannelAddress(ctx, domain.ChannelTelegram, addr)
	if err != nil {
		t.Fatalf("SubscribersByChannelAddress: %v", err)
	}
	if len(left) != 0 {
		t.Fatalf("подписок не должно остаться: %+v", left)
	}
}
