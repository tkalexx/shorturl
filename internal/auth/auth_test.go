package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSignAndVerify(t *testing.T) {
	m := NewManager("secret")
	value := m.sign("user-123")
	id, err := m.verify(value)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if id != "user-123" {
		t.Fatalf("got %q", id)
	}

	if _, err := m.verify("broken"); err == nil {
		t.Fatal("expected error for broken cookie")
	}
	if _, err := m.verify("|abcdef"); err == nil {
		t.Fatal("expected error for empty user id")
	}
	if _, err := m.verify("user|zzzz"); err == nil {
		t.Fatal("expected error for bad hex")
	}
	if _, err := m.verify("user-123|0000000000000000000000000000000000000000000000000000000000000000"); err == nil {
		t.Fatal("expected error for wrong signature")
	}
}

func TestMiddlewareIssuesCookie(t *testing.T) {
	m := NewManager("secret")
	h := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := UserIDFromContext(r.Context())
		if err != nil || id == "" {
			t.Errorf("missing user id in context: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status=%d", rr.Code)
	}
	if len(rr.Result().Cookies()) == 0 {
		t.Fatal("expected Set-Cookie")
	}
}

func TestMiddlewareUnauthorizedEmptyUser(t *testing.T) {
	m := NewManager("secret")
	h := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: CookieName, Value: ""})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestMiddlewareReissuesOnBadSignature(t *testing.T) {
	m := NewManager("secret")
	h := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: CookieName, Value: "uid|0000000000000000000000000000000000000000000000000000000000000000"})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if len(rr.Result().Cookies()) == 0 {
		t.Fatal("expected reissued cookie")
	}
}

func TestUserIDFromContext(t *testing.T) {
	if _, err := UserIDFromContext(context.Background()); err == nil {
		t.Fatal("expected error")
	}
	ctx := WithUserID(context.Background(), "x")
	id, err := UserIDFromContext(ctx)
	if err != nil || id != "x" {
		t.Fatalf("got id=%q err=%v", id, err)
	}
}
