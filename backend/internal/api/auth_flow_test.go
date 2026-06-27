package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/healthpage/backend/internal/auth"
	"github.com/healthpage/backend/internal/domain"
	"github.com/healthpage/backend/internal/security"
	"github.com/healthpage/backend/internal/store"
)

// fakeRepo — in-memory реализация auth.Repo для тестов без БД.
type fakeRepo struct {
	users   map[uuid.UUID]domain.User
	byEmail map[string]uuid.UUID
	refresh map[string]store.RefreshTokenRecord
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		users:   map[uuid.UUID]domain.User{},
		byEmail: map[string]uuid.UUID{},
		refresh: map[string]store.RefreshTokenRecord{},
	}
}

func (f *fakeRepo) CreateUserWithAccount(_ context.Context, email, hash, name, _, locale string) (domain.User, domain.Account, error) {
	if _, ok := f.byEmail[email]; ok {
		return domain.User{}, domain.Account{}, store.ErrEmailTaken
	}
	u := domain.User{ID: uuid.New(), Email: email, PasswordHash: hash, Name: name, Locale: locale, IsActive: true}
	f.users[u.ID] = u
	f.byEmail[email] = u.ID
	return u, domain.Account{ID: uuid.New(), OwnerUserID: u.ID}, nil
}

func (f *fakeRepo) UserByEmail(_ context.Context, email string) (domain.User, error) {
	if id, ok := f.byEmail[email]; ok {
		return f.users[id], nil
	}
	return domain.User{}, store.ErrNotFound
}

func (f *fakeRepo) UserByID(_ context.Context, id uuid.UUID) (domain.User, error) {
	if u, ok := f.users[id]; ok {
		return u, nil
	}
	return domain.User{}, store.ErrNotFound
}

func (f *fakeRepo) CreateRefreshToken(_ context.Context, userID uuid.UUID, hash string, exp time.Time) error {
	f.refresh[hash] = store.RefreshTokenRecord{UserID: userID, ExpiresAt: exp}
	return nil
}

func (f *fakeRepo) RefreshTokenByHash(_ context.Context, hash string) (store.RefreshTokenRecord, error) {
	if rec, ok := f.refresh[hash]; ok {
		return rec, nil
	}
	return store.RefreshTokenRecord{}, store.ErrNotFound
}

func (f *fakeRepo) RevokeRefreshToken(_ context.Context, hash string) error {
	if rec, ok := f.refresh[hash]; ok {
		now := time.Now()
		rec.RevokedAt = &now
		f.refresh[hash] = rec
	}
	return nil
}

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	tm, err := security.NewTokenManager("test-secret", 15*time.Minute, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	svc := auth.NewService(newFakeRepo(), tm)
	return httptest.NewServer(NewRouter(Deps{Auth: svc, RefreshTTL: time.Hour}))
}

func postJSON(t *testing.T, c *http.Client, url string, body any) *http.Response {
	t.Helper()
	buf, _ := json.Marshal(body)
	resp, err := c.Post(url, "application/json", bytes.NewReader(buf))
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	return resp
}

func TestAuthFlow(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()
	jar, _ := cookiejar.New(nil)
	c := &http.Client{Jar: jar}
	base := srv.URL + "/api/v1/auth"

	// register
	resp := postJSON(t, c, base+"/register", map[string]string{
		"email": "Op@Example.ru", "password": "supersecret", "name": "Оп",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("register: want 201, got %d", resp.StatusCode)
	}
	var reg authResultResponse
	json.NewDecoder(resp.Body).Decode(&reg)
	resp.Body.Close()
	if reg.AccessToken == "" || reg.TokenType != "Bearer" {
		t.Fatalf("register: bad token payload %+v", reg)
	}
	if reg.RefreshToken != nil {
		t.Fatal("refresh не должен возвращаться в теле (только cookie)")
	}

	// /me с токеном
	meReq, _ := http.NewRequest(http.MethodGet, base+"/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+reg.AccessToken)
	meResp, _ := c.Do(meReq)
	if meResp.StatusCode != http.StatusOK {
		t.Fatalf("me: want 200, got %d", meResp.StatusCode)
	}
	var me authUser
	json.NewDecoder(meResp.Body).Decode(&me)
	meResp.Body.Close()
	if me.Email != "op@example.ru" {
		t.Fatalf("me: email normalized? got %q", me.Email)
	}

	// /me без токена -> 401
	noTok, _ := http.Get(base + "/me")
	if noTok.StatusCode != http.StatusUnauthorized {
		t.Fatalf("me no token: want 401, got %d", noTok.StatusCode)
	}
	noTok.Body.Close()

	// duplicate register -> 409
	dup := postJSON(t, c, base+"/register", map[string]string{"email": "op@example.ru", "password": "supersecret"})
	if dup.StatusCode != http.StatusConflict {
		t.Fatalf("dup register: want 409, got %d", dup.StatusCode)
	}
	dup.Body.Close()

	// login wrong password -> 401
	bad := postJSON(t, c, base+"/login", map[string]string{"email": "op@example.ru", "password": "nope"})
	if bad.StatusCode != http.StatusUnauthorized {
		t.Fatalf("bad login: want 401, got %d", bad.StatusCode)
	}
	bad.Body.Close()

	// login correct -> 200
	good := postJSON(t, c, base+"/login", map[string]string{"email": "op@example.ru", "password": "supersecret"})
	if good.StatusCode != http.StatusOK {
		t.Fatalf("login: want 200, got %d", good.StatusCode)
	}
	good.Body.Close()

	// refresh по cookie -> 200 (ротация)
	ref := postJSON(t, c, base+"/refresh", nil)
	if ref.StatusCode != http.StatusOK {
		t.Fatalf("refresh: want 200, got %d", ref.StatusCode)
	}
	ref.Body.Close()

	// logout -> 204
	out := postJSON(t, c, base+"/logout", nil)
	if out.StatusCode != http.StatusNoContent {
		t.Fatalf("logout: want 204, got %d", out.StatusCode)
	}
	out.Body.Close()

	// refresh после logout -> 401
	after := postJSON(t, c, base+"/refresh", nil)
	if after.StatusCode != http.StatusUnauthorized {
		t.Fatalf("refresh after logout: want 401, got %d", after.StatusCode)
	}
	after.Body.Close()
}
